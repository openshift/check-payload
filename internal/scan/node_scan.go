package scan

import (
	"context"
	"os"
	"path/filepath"

	"k8s.io/klog/v2"

	"github.com/openshift/check-payload/internal/rpm"
	"github.com/openshift/check-payload/internal/types"
	"github.com/openshift/check-payload/internal/validations"
)

func RunNodeScan(ctx context.Context, cfg *types.Config, root string) []*types.ScanResults {
	klog.Info("scanning node")
	return []*types.ScanResults{rpmRootScan(ctx, cfg, root)}
}

func rpmRootScan(ctx context.Context, cfg *types.Config, root string) *types.ScanResults {
	results := types.NewScanResults()
	rpms, err := rpm.GetAllRPMs(ctx, root)
	if err != nil {
		results.Append(types.NewScanResult().SetError(err))
		return results
	}
	for _, pkg := range rpms {
		files, err := rpm.GetFilesFromRPM(ctx, root, pkg.NVRA)
		if err != nil {
			res := types.NewScanResult().SetRPM(pkg.Name).SetError(err)
			results.Append(res)
			continue
		}
		for _, innerPath := range files {
			if cfg.IgnoreFile(innerPath) || cfg.IgnoreDirPrefix(innerPath) || cfg.IgnoreFileByRpm(innerPath, pkg.Name) {
				continue
			}
			path := filepath.Join(root, innerPath)
			fileInfo, err := os.Lstat(path)
			if err != nil {
				// some files are stripped from an rhcos image
				continue
			}
			if m := fileInfo.Mode(); !m.IsRegular() || m.Perm()&0o111 == 0 {
				// Skip all non-regular files (directories, symlinks),
				// and regular files that has no x bit set.
				continue
			}
			klog.V(1).InfoS("scanning path", "path", innerPath)
			res := validations.ScanBinary(ctx, root, innerPath, cfg.RPMIgnores, cfg.ErrIgnores)
			if res.Skip {
				// Do not add skipped binaries to results.
				continue
			}
			if res.IsSuccess() {
				klog.V(1).InfoS("scanning node success", "path", innerPath, "status", "success")
			} else {
				status := res.Status()
				klog.InfoS("scanning node "+status,
					"rpm", res.RPM,
					"path", innerPath,
					"error", res.Error,
					"status", status)
			}
			results.Append(res)
		}
	}
	return results
}
