package scan

import (
	"context"
	"os"
	"path/filepath"

	v1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/openshift/check-payload/internal/rpm"
	"github.com/openshift/check-payload/internal/types"
	"github.com/openshift/check-payload/internal/validations"
)

func NewTag(name string) *v1.TagReference {
	return &v1.TagReference{
		From: &corev1.ObjectReference{
			Name: name,
		},
	}
}

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
		tag := NewTag(pkg)
		files, err := rpm.GetFilesFromRPM(ctx, root, pkg)
		if err != nil {
			res := types.NewScanResult().SetTag(tag).SetError(err)
			results.Append(res)
			continue
		}
		for _, innerPath := range files {
			if cfg.IgnoreFile(innerPath) || cfg.IgnoreDirPrefix(innerPath) || cfg.IgnoreFileByRpm(innerPath, pkg) {
				continue
			}
			path := filepath.Join(cfg.NodeScan, innerPath)
			fileInfo, err := os.Lstat(path)
			if err != nil {
				// some files are stripped from an rhcos image
				continue
			}
			if fileInfo.IsDir() {
				continue
			}
			if fileInfo.Mode()&os.ModeSymlink != 0 {
				continue
			}
			klog.V(1).InfoS("scanning path", "path", path)
			res := validations.ScanBinary(ctx, component, tag, root, innerPath)
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
