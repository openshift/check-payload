package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gabriel-vasile/mimetype"
	v1 "github.com/openshift/api/image/v1"
	"github.com/openshift/oc/pkg/cli/admin/release"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

func validateApplicationDependencies(apps []string) error {
	var multiErr error

	for _, app := range apps {
		if _, err := exec.LookPath(app); err != nil {
			multierr.AppendInto(&multiErr, fmt.Errorf("executable not found: %v", err))
		}
	}

	return multiErr
}

type Request struct {
	Tag *v1.TagReference
}

type Result struct {
	Tag     *v1.TagReference
	Results *ScanResults
}

func runOperatorScan(ctx context.Context, cfg *Config) []*ScanResults {
	tag := &v1.TagReference{
		From: &corev1.ObjectReference{
			Name: cfg.ContainerImage,
		},
	}

	results := validateTag(ctx, tag, cfg)

	var runs []*ScanResults
	runs = append(runs, results)
	return runs
}

func runPayloadScan(ctx context.Context, cfg *Config) []*ScanResults {
	var runs []*ScanResults

	payload, err := GetPayload(cfg)
	if err != nil {
		klog.Fatalf("could not get pods from payload: %v", err)
	}

	parallelism := cfg.Parallelism
	limit := cfg.Limit

	tx := make(chan *Request, parallelism)
	rx := make(chan *Result, parallelism)
	var wgThreads sync.WaitGroup
	var wgRx sync.WaitGroup

	wgThreads.Add(cfg.Parallelism)
	for i := 0; i < parallelism; i++ {
		go func() {
			scan(ctx, cfg, tx, rx)
			wgThreads.Done()
		}()
	}

	wgRx.Add(1)
	go func() {
		for res := range rx {
			runs = append(runs, res.Results)
		}
		wgRx.Done()
	}()

	contains := func(slice []string, item string) bool {
		for _, elem := range slice {
			if elem == item {
				return true
			}
		}
		return false
	}

	for i, tag := range payload.References.Spec.Tags {
		// scan only user specified components if provided
		// on command line
		if len(cfg.Components) > 0 && !contains(cfg.Components, tag.Name) {
			continue
		}
		tag := tag
		tx <- &Request{Tag: &tag}
		if limit > 0 && int(i) == limit-1 {
			break
		}
	}

	close(tx)
	wgThreads.Wait()
	close(rx)
	wgRx.Wait()

	return runs
}

func scan(ctx context.Context, cfg *Config, tx <-chan *Request, rx chan<- *Result) {
	for req := range tx {
		ValidateTag(ctx, cfg, req.Tag, rx)
	}
}

func ValidateTag(ctx context.Context, cfg *Config, tag *v1.TagReference, rx chan<- *Result) {
	result := validateTag(ctx, tag, cfg)
	rx <- &Result{Results: result}
}

func isFailed(results []*ScanResults) bool {
	for _, result := range results {
		for _, res := range result.Items {
			if res.Error != nil {
				return true
			}
		}
	}
	return false
}

func GetPayload(config *Config) (*release.ReleaseInfo, error) {
	var payload *release.ReleaseInfo
	var err error
	if config.FromURL != "" {
		payload, err = DownloadReleaseInfo(config.FromURL)
	} else {
		payload, err = ReadReleaseInfo(config.FromFile)
	}
	return payload, err
}

func DownloadReleaseInfo(url string) (*release.ReleaseInfo, error) {
	// oc adm release info  --output json --pullspecs
	klog.InfoS("oc adm release info", "url", url)
	var stdout bytes.Buffer
	cmd := exec.CommandContext(context.Background(), "oc", "adm", "release", "info", "--output", "json", "--pullspecs", url)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	releaseInfo := &release.ReleaseInfo{}
	if err := json.Unmarshal(stdout.Bytes(), releaseInfo); err != nil {
		return nil, err
	}
	return releaseInfo, nil
}

func ReadReleaseInfo(filename string) (*release.ReleaseInfo, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	releaseInfo := &release.ReleaseInfo{}
	if err := json.Unmarshal([]byte(data), releaseInfo); err != nil {
		return nil, err
	}
	return releaseInfo, nil
}

func validateTag(ctx context.Context, tag *v1.TagReference, cfg *Config) *ScanResults {
	results := NewScanResults()

	image := tag.From.Name

	// skip over ignored images
	for _, ignoredImage := range cfg.FilterImages {
		if ignoredImage == image {
			klog.InfoS("Ignoring image", "image", image)
			results.Append(NewScanResult().SetTag(tag).Success())
			return results
		}
	}

	// pull
	if err := podmanPull(ctx, image, cfg.InsecurePull); err != nil {
		results.Append(NewScanResult().SetTag(tag).SetError(err))
		return results
	}
	// create
	createID, err := podmanCreate(ctx, image)
	if err != nil {
		results.Append(NewScanResult().SetTag(tag).SetError(err))
		return results
	}
	// mount
	mountPath, err := podmanMount(ctx, createID)
	if err != nil {
		results.Append(NewScanResult().SetTag(tag).SetError(err))
		return results
	}
	// get openshift component
	component, _ := getOpenshiftComponentFromImage(ctx, image)
	if component != "" {
		klog.InfoS("found operator", "component", component)
	}
	defer func() {
		podmanUnmount(ctx, createID)
	}()

	// does the image contain openssl
	opensslInfo := validateOpenssl(ctx, mountPath)
	results.Append(NewScanResult().SetOpenssl(opensslInfo).SetTag(tag))

	// business logic for scan
	if err := filepath.WalkDir(mountPath, func(path string, file fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		printablePath := stripMountPath(mountPath, path)
		if isPathFiltered(cfg.FilterPaths, printablePath) {
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
		klog.InfoS("scanning path", "path", path)
		res := scanBinary(ctx, component, tag, mountPath, path)
		if res.Error == nil {
			klog.InfoS("scanning success", "image", image, "path", printablePath, "status", "success")
		} else {
			klog.InfoS("scanning failed", "image", image, "path", printablePath, "error", res.Error, "status", "failed")
		}
		results.Append(res)
		return nil
	}); err != nil {
		return results
	}

	return results
}

func isPathFiltered(filterPaths []string, path string) bool {
	for _, filter := range filterPaths {
		if strings.HasPrefix(path, filter) {
			return true
		}
	}
	return false
}

func stripMountPath(mountPath, path string) string {
	return strings.TrimPrefix(path, mountPath)
}
