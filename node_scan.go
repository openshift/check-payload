package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	v1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

func NewTag(name string) *v1.TagReference {
	return &v1.TagReference{
		From: &corev1.ObjectReference{
			Name: name,
		},
	}
}

func runNodeScan(ctx context.Context, cfg *Config) []*ScanResults {
	var runs []*ScanResults
	results := NewScanResults()
	runs = append(runs, results)
	klog.Info("scanning node")
	rpms, _ := getAllRPMs(ctx, cfg)
	for _, rpm := range rpms {
		tag := NewTag(rpm)
		files, err := getFilesFromRPM(ctx, cfg, rpm)
		if err != nil {
			res := NewScanResult().SetTag(tag).SetError(err)
			results.Append(res)
			continue
		}
		for _, innerPath := range files {
			if cfg.IgnoreFile(innerPath) || cfg.IgnoreDirPrefix(innerPath) {
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
			tag = NewTag(path)
			klog.V(1).InfoS("scanning path", "path", path)
			component := &OpenshiftComponent{
				Component: "node",
			}
			res := scanBinary(ctx, component, tag, cfg.NodeScan, innerPath)
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

func getFilesFromRPM(ctx context.Context, cfg *Config, rpm string) ([]string, error) {
	klog.Infof("rpm -ql %v", rpm)
	files := []string{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "rpm", "-ql", "--root", cfg.NodeScan, rpm)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return files, fmt.Errorf("rpm -ql error (stderr=%v) (err=%v)", stderr.String(), err)
	}

	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		files = append(files, scanner.Text())
	}
	return files, nil
}

func getAllRPMs(ctx context.Context, cfg *Config) ([]string, error) {
	klog.Info("rpm -qa")
	rpms := []string{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "rpm", "-qa", "--root", cfg.NodeScan)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return rpms, fmt.Errorf("rpm -qa error (stderr=%v) (err=%v)", stderr.String(), err)
	}

	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		rpms = append(rpms, scanner.Text())
	}
	return rpms, nil
}
