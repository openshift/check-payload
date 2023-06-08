package main

import (
	"context"
	"io/fs"
	"path/filepath"

	"github.com/gabriel-vasile/mimetype"
	v1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

func runNodeScan(ctx context.Context, cfg *Config) []*ScanResults {
	var runs []*ScanResults
	results := NewScanResults()
	runs = append(runs, results)
	klog.Info("scanning node")
	cfg.Filter = append(cfg.Filter,
		"/proc",
		"/sys",
		"/dev",
		"/usr/lib/firmware",
		"/usr/lib/modules",
		"/usr/lib/grub",
	)
	// business logic for scan
	if err := filepath.WalkDir("/", func(path string, file fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if isPathFiltered(cfg.Filter, path) {
			return nil
		}
		if file.IsDir() {
			return nil
		}
		if !file.Type().IsRegular() {
			return nil
		}
		mtype, err := mimetype.DetectFile(path)
		if err != nil {
			return err
		}
		if mimetype.EqualsAny(mtype.String(), ignoredMimes...) {
			return nil
		}
		tag := &v1.TagReference{
			From: &corev1.ObjectReference{
				Name: "node",
			},
		}
		klog.InfoS("scanning path", "path", path, "mtype", mtype.String())
		res := scanBinary(ctx, tag, "/", path)
		if res.Error == nil {
			klog.InfoS("scanning node success", "path", path, "status", "success")
		} else {
			klog.InfoS("scanning node failed", "path", path, "error", res.Error, "status", "failed")
		}
		results.Append(res)
		return nil
	}); err != nil {
		klog.Fatalf("%v", err)
		return runs
	}
	return runs
}
