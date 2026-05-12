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
	version, present := artifactPresentAndVersion(ctx, mountPath, m)
	if !present {
		klog.V(1).InfoS("fips artifact missing", "artifact", m.CertifiedArtifact, "module", m.Module, "mountPath", mountPath)
		return types.NewValidationError(fmt.Errorf("%s: %w", m.CertifiedArtifact, types.ErrFipsArtifactMissing))
	}
	if m.CertifiedArtifactMinVersion != "" && !types.VersionAtLeast(version, m.CertifiedArtifactMinVersion) {
		installed := version
		if installed == "" {
			installed = "unknown (detected via file path, RPM not available)"
		}
		klog.V(1).InfoS("fips artifact version too low", "artifact", m.CertifiedArtifact, "installed", installed, "required", m.CertifiedArtifactMinVersion)
		return types.NewValidationError(fmt.Errorf("%s (installed %s, need >= %s): %w",
			m.CertifiedArtifact, installed, m.CertifiedArtifactMinVersion, types.ErrFipsArtifactVersionLow))
	}
	klog.V(1).InfoS("fips artifact present", "artifact", m.CertifiedArtifact, "version", version, "module", m.Module)
	return nil
}

func artifactPresentAndVersion(ctx context.Context, mountPath string, m types.FipsModule) (version string, present bool) {
	rpms, err := rpm.GetAllRPMs(ctx, mountPath)
	if err == nil {
		for _, r := range rpms {
			if r.Name == m.CertifiedArtifact {
				v, err := rpm.VersionOf(ctx, mountPath, m.CertifiedArtifact)
				if err == nil {
					klog.V(1).InfoS("fips artifact found via RPM", "artifact", m.CertifiedArtifact, "version", v)
					return v, true
				}
				klog.V(1).InfoS("fips artifact RPM found but version query failed", "artifact", m.CertifiedArtifact, "error", err)
				return "", true
			}
		}
	}
	for _, p := range m.CertifiedArtifactPaths {
		full := filepath.Join(mountPath, p)
		if _, err := os.Stat(full); err == nil {
			klog.V(1).InfoS("fips artifact found via file path", "artifact", m.CertifiedArtifact, "path", p)
			return "", true
		}
	}
	return "", false
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

// ValidateHostLibsForModules checks that each detected module's host library
// exports FIPS symbols. Only modules registered in moduleHostLibChecks are
// checked; modules without a host library (e.g. Go native FIPS) are skipped.
func ValidateHostLibsForModules(ctx context.Context, mountPath string, modulesInUse map[string]bool) []*types.ValidationError {
	var errs []*types.ValidationError
	for module := range modulesInUse {
		check, ok := moduleHostLibChecks[module]
		if !ok {
			continue
		}
		if ve := validateHostLib(ctx, mountPath, module, check); ve != nil {
			errs = append(errs, ve)
		}
	}
	return errs
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
