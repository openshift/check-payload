package validations

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/openshift/check-payload/internal/rpm"
	"github.com/openshift/check-payload/internal/types"

	"k8s.io/klog/v2"
)

func CheckArtifact(ctx context.Context, m types.FipsModule, mountPath string) *types.ValidationError {
	version, present := rpmPresentAndVersion(ctx, mountPath, m.CertifiedArtifact)
	if !present {
		klog.V(1).InfoS("fips artifact RPM missing", "artifact", m.CertifiedArtifact, "module", m.Module)
		return types.NewValidationError(fmt.Errorf("%s: %w", m.CertifiedArtifact, types.ErrFipsArtifactMissing))
	}
	if version != "" {
		atLeast, atMost := types.VersionInRange(version, m.CertifiedArtifactMinVersion, m.CertifiedArtifactMaxVersion)
		if !atLeast {
			klog.V(1).InfoS("fips artifact version too low", "artifact", m.CertifiedArtifact, "installed", version, "required", m.CertifiedArtifactMinVersion)
			return types.NewValidationError(fmt.Errorf("%s (installed %s, need >= %s): %w",
				m.CertifiedArtifact, version, m.CertifiedArtifactMinVersion, types.ErrFipsArtifactVersionLow))
		}
		if !atMost {
			klog.V(1).InfoS("fips artifact version exceeds certified range", "artifact", m.CertifiedArtifact, "installed", version, "maxCertified", m.CertifiedArtifactMaxVersion)
			return types.NewValidationError(fmt.Errorf("%s (installed %s, certified <= %s): %w",
				m.CertifiedArtifact, version, m.CertifiedArtifactMaxVersion, types.ErrFipsArtifactVersionHigh))
		}
	}
	if len(m.CertifiedArtifactPaths) > 0 && !anyPathExists(mountPath, m.CertifiedArtifactPaths) {
		klog.V(1).InfoS("fips artifact RPM present but certified file missing", "artifact", m.CertifiedArtifact, "paths", m.CertifiedArtifactPaths)
		return types.NewValidationError(fmt.Errorf("%s RPM present but certified file not found at %v: %w",
			m.CertifiedArtifact, m.CertifiedArtifactPaths, types.ErrFipsArtifactMissing))
	}
	klog.V(1).InfoS("fips artifact present", "artifact", m.CertifiedArtifact, "version", version, "module", m.Module)
	return nil
}

func rpmPresentAndVersion(ctx context.Context, mountPath, rpmName string) (version string, present bool) {
	rpms, err := rpm.GetAllRPMs(ctx, mountPath)
	if err != nil {
		return "", false
	}
	for _, r := range rpms {
		if r.Name == rpmName {
			v, err := rpm.VersionOf(ctx, mountPath, rpmName)
			if err == nil {
				klog.V(1).InfoS("fips artifact found via RPM", "artifact", rpmName, "version", v)
				return v, true
			}
			klog.V(1).InfoS("fips artifact RPM found but version query failed", "artifact", rpmName, "error", err)
			return "", true
		}
	}
	return "", false
}

func anyPathExists(mountPath string, paths []string) bool {
	for _, p := range paths {
		if _, err := os.Stat(filepath.Join(mountPath, p)); err == nil {
			return true
		}
	}
	return false
}

type hostLibCheck struct {
	lib         string
	fipsSymbols []string
}

// moduleHostLibChecks maps module names to host library FIPS requirements.
// Add entries here when a new crypto stack needs host library validation.
var moduleHostLibChecks = map[string]hostLibCheck{
	"openssl": {
		lib:         "libcrypto.so",
		fipsSymbols: []string{"FIPS_mode", "fips_mode", "EVP_default_properties_is_fips_enabled"},
	},
}

var hostLibSearchPaths = []string{"/usr/lib64", "/usr/lib"}

func findLib(mountPath string, searchPaths []string, subname string) (string, error) {
	for _, dir := range searchPaths {
		files, err := os.ReadDir(filepath.Join(mountPath, dir))
		if err != nil {
			continue
		}
		for _, file := range files {
			if strings.Contains(file.Name(), subname) && !strings.Contains(file.Name(), "hmac") {
				return filepath.Join(dir, file.Name()), nil
			}
		}
	}
	return "", fmt.Errorf("%s not found", subname)
}

// ValidateModule performs unified FIPS validation for a single module.
// Binary-source modules are skipped (validated during binary inspection).
// Image-source modules try artifact check first, then fall back to host lib
// FIPS symbol check. Passes if either succeeds.
func ValidateModule(ctx context.Context, cfg *types.Config, mountPath string, module string) *types.ValidationError {
	for _, r := range cfg.GetFIPSCertifiedModules() {
		if r.Module == module && r.IsBinarySource() {
			klog.V(1).InfoS("fips module validated at binary level, skipping image check", "module", module)
			return nil
		}
	}

	for _, r := range cfg.GetFIPSCertifiedModules() {
		if r.Module != module || r.CertifiedArtifact == "" {
			continue
		}
		klog.V(1).InfoS("checking fips artifact", "module", r.Module, "artifact", r.CertifiedArtifact)
		if ve := CheckArtifact(ctx, r, mountPath); ve == nil {
			return nil
		}
	}

	if check, ok := moduleHostLibChecks[module]; ok {
		if ve := validateHostLib(ctx, mountPath, module, check); ve == nil {
			return nil
		}
	}

	return types.NewValidationError(fmt.Errorf("no FIPS certified artifact or library found for module %s: %w", module, types.ErrFipsArtifactMissing))
}

func validateHostLib(ctx context.Context, mountPath, module string, check hostLibCheck) *types.ValidationError {
	libPath, err := findLib(mountPath, hostLibSearchPaths, check.lib)
	if err != nil {
		return types.NewValidationError(fmt.Errorf("%s host library not present: %w", module, err))
	}

	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "nm", "-D", filepath.Join(mountPath, libPath))
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return types.NewValidationError(fmt.Errorf("failed to inspect %s: %w", libPath, err))
	}

	out := stdout.Bytes()
	for _, sym := range check.fipsSymbols {
		if bytes.Contains(out, []byte(sym)) {
			klog.V(1).InfoS("host lib FIPS check passed", "module", module, "lib", libPath, "symbol", sym)
			return nil
		}
	}

	return types.NewValidationError(fmt.Errorf("%s is missing FIPS symbols %v", libPath, check.fipsSymbols))
}
