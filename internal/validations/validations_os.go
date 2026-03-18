package validations

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/openshift/check-payload/internal/types"

	"k8s.io/klog/v2"
)

const releaseFilePath = "/etc/redhat-release"

func ValidateOS(cfg *types.Config, mountPath string) (info types.OSInfo) {
	info.Path = releaseFilePath

	cd := cfg.GetCertifiedDistributions()
	if len(cd) == 0 {
		info.Error = types.NewValidationError(types.ErrCertifiedDistributionsEmpty).SetWarning()
		return info
	}

	path, err := GetTargetPath(mountPath)
	if err != nil {
		info.Error = types.NewValidationError(err)
		return info
	}

	f, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			info.Error = types.NewValidationError(types.ErrDistributionFileMissing)
		} else {
			info.Error = types.NewValidationError(err)
		}
		return info
	}
	if len(f) == 0 {
		info.Error = types.NewValidationError(fmt.Errorf("%v is an empty file", releaseFilePath))
		return info
	}

	for _, d := range cd {
		if bytes.HasPrefix(f, []byte(d)) {
			info.Certified = true
			break
		}
	}
	return info
}

// ValidateModuleArtifacts checks that each detected module's certified
// artifact is present. Runs independently of the OS allowlist check.
func ValidateModuleArtifacts(ctx context.Context, cfg *types.Config, mountPath string, modulesInUse []string) *types.ValidationError {
	for _, r := range cfg.GetFIPSCertifiedModules() {
		if r.CertifiedArtifact == "" {
			continue
		}
		if !containsStr(modulesInUse, r.Module) {
			klog.V(1).InfoS("fips module not in use, skipping", "module", r.Module, "artifact", r.CertifiedArtifact)
			continue
		}
		klog.V(1).InfoS("fips module in use, checking artifact", "module", r.Module, "artifact", r.CertifiedArtifact, "minVersion", r.CertifiedArtifactMinVersion)
		if ve := CheckArtifact(ctx, r, mountPath); ve != nil {
			return ve
		}
	}
	return nil
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// in case the file is symlinked, we need to check to ensure there is not a target path
func GetTargetPath(mountPath string) (string, error) {
	path := filepath.Join(mountPath, releaseFilePath)
	fi, err := os.Lstat(path)
	if err != nil {
		return path, err
	}
	isSymlink := fi.Mode()&fs.ModeSymlink != 0
	if isSymlink {
		linkTarget, err := os.Readlink(path)
		if err != nil {
			return path, err
		}

		var targetPath string
		if filepath.IsAbs(linkTarget) {
			// If the symlink target is absolute (e.g., "/usr/lib/system-release"),
			// join it with the mount path to get the actual file location
			targetPath = filepath.Join(mountPath, linkTarget)
		} else {
			// If the symlink target is relative (e.g., "../usr/lib/system-release"),
			// resolve it relative to the symlink's directory
			symlinkDir := filepath.Dir(path)
			targetPath = filepath.Join(symlinkDir, linkTarget)
		}
		return targetPath, nil
	}
	return path, nil
}
