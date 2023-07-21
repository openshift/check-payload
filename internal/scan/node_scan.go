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

func RunNodeScan(ctx context.Context, cfg *types.Config) []*types.ScanResults {
	var runs []*types.ScanResults
	results := types.NewScanResults()
	runs = append(runs, results)
	klog.Info("scanning node")
	component := &types.OpenshiftComponent{
		Component: "node",
	}
	root := cfg.NodeScan
	rpms, _ := rpm.GetAllRPMs(ctx, root)
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
			path := filepath.Join(cfg.NodeScan, innerPath)
			fileInfo, err := os.Lstat(path)
			if err != nil {
				// some files are stripped from an rhcos image
				continue
			}
			if m := fileInfo.Mode(); !m.IsRegular() {
				// Skip all non-regular files (directories, symlinks).
				continue
			}
			klog.V(1).InfoS("scanning path", "path", path)
			res := validations.ScanBinary(ctx, component, nil, root, innerPath)
			if res.Skip {
				// Do not add skipped binaries to results.
				continue
			}
			if res.Error == nil {
				klog.V(1).InfoS("scanning node success", "path", path, "status", "success")
			} else {
				klog.InfoS("scanning node failed", "path", path, "error", res.Error, "status", "failed")
			}
			results.Append(res)
		}
	}
	return runs
}
