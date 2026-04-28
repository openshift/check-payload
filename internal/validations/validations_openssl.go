package validations

import (
	"bytes"
	"context"
	"errors"
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

func findLib(mountPath string, searchPaths []string, subname string) (path string, err error) {
	var returnPath string
	for _, path := range searchPaths {
		files, err := os.ReadDir(filepath.Join(mountPath, path))
		if err != nil {
			continue
		}
		for _, file := range files {
			if strings.Contains(file.Name(), subname) && !strings.Contains(file.Name(), "hmac") {
				returnPath = filepath.Join(path, file.Name())
				break
			}
		}
	}
	if returnPath == "" {
		return "", errors.New("openssl not found")
	}
	return returnPath, nil
}

func ValidateOpenssl(ctx context.Context, mountPath string) types.OpensslInfo {
	info := types.OpensslInfo{
		Present: false,
		FIPS:    false,
		Error:   nil,
	}

	path, err := findLib(mountPath, []string{"/usr/lib64", "/usr/lib"}, "libcrypto.so")
	if err != nil {
		info.Present = false
		info.FIPS = false
		return info
	}
	info.Path = path

	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "nm", "-D", filepath.Join(mountPath, path))
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		info.Error = err
		return info
	}

	info.Present = true
	info.FIPS = bytes.Contains(stdout.Bytes(), []byte("FIPS_mode")) || bytes.Contains(stdout.Bytes(), []byte("fips_mode")) || bytes.Contains(stdout.Bytes(), []byte("EVP_default_properties_is_fips_enabled"))

	return info
}
