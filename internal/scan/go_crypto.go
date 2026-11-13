package scan

import (
	"context"
	"io/fs"
	"path/filepath"

	v1 "github.com/openshift/api/image/v1"
	"github.com/openshift/check-payload/internal/types"
	"github.com/openshift/check-payload/internal/validations"
)

// scanGoCryptoPhase shares image traversal configuration and result formatting
// with the scanner, but also examines transported executables without execute bits.
func scanGoCryptoPhase(ctx context.Context, cfg *types.Config, tag *v1.TagReference, component *types.OpenshiftComponent, mountPath string, results *types.ScanResults) {
	if !cfg.DetectGoCrypto || len(cfg.GoCryptoTargets) == 0 {
		return
	}
	err := filepath.WalkDir(mountPath, func(path string, file fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		innerPath := stripMountPath(mountPath, path)
		if file.IsDir() {
			if cfg.IgnoreDirWithComponent(innerPath, component) {
				return filepath.SkipDir
			}
			return nil
		}
		if !file.Type().IsRegular() || cfg.IgnoreFileWithTag(innerPath, tag) || cfg.IgnoreFileWithComponent(innerPath, component) {
			return nil
		}
		if res := validations.ScanGoCryptoTargets(mountPath, innerPath, cfg.GoCryptoTargets); res != nil {
			results.Append(res.SetTag(tag).SetComponent(component))
		}
		return nil
	})
	if err != nil {
		results.Append(types.NewScanResult().SetError(err).SetTag(tag).SetComponent(component))
	}
}
