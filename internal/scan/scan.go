package scan

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/openshift/check-payload/internal/podman"
	"github.com/openshift/check-payload/internal/types"
	"github.com/openshift/check-payload/internal/validations"

	v1 "github.com/openshift/api/image/v1"
	"github.com/openshift/oc/pkg/cli/admin/release"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type Request struct {
	Tag *v1.TagReference
}

type Result struct {
	Tag     *v1.TagReference
	Results *types.ScanResults
}

func ValidateApplicationDependencies(apps []string) error {
	var multiErr error

	for _, app := range apps {
		if _, err := exec.LookPath(app); err != nil {
			multierr.AppendInto(&multiErr, err)
		}
	}

	return multiErr
}

func RunOperatorScan(ctx context.Context, cfg *types.Config) []*types.ScanResults {
	tag := &v1.TagReference{
		From: &corev1.ObjectReference{
			Name: cfg.ContainerImage,
		},
	}

	results := validateTag(ctx, tag, cfg)

	var runs []*types.ScanResults
	runs = append(runs, results)
	return runs
}

func RunPayloadScan(ctx context.Context, cfg *types.Config) []*types.ScanResults {
	var runs []*types.ScanResults

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
		if limit > 0 && i == limit-1 {
			break
		}
	}

	close(tx)
	wgThreads.Wait()
	close(rx)
	wgRx.Wait()

	return runs
}

func scan(ctx context.Context, cfg *types.Config, tx <-chan *Request, rx chan<- *Result) {
	for req := range tx {
		ValidateTag(ctx, cfg, req.Tag, rx)
	}
}

func ValidateTag(ctx context.Context, cfg *types.Config, tag *v1.TagReference, rx chan<- *Result) {
	result := validateTag(ctx, tag, cfg)
	rx <- &Result{Results: result}
}

func IsFailed(results []*types.ScanResults) bool {
	for _, result := range results {
		for _, res := range result.Items {
			if res.IsLevel(types.Error) {
				return true
			}
		}
	}
	return false
}

func IsWarnings(results []*types.ScanResults) bool {
	for _, result := range results {
		for _, res := range result.Items {
			if res.IsLevel(types.Warning) {
				return true
			}
		}
	}
	return false
}

func GetPayload(config *types.Config) (*release.ReleaseInfo, error) {
	var payload *release.ReleaseInfo
	var err error
	if config.FromURL != "" {
		payload, err = DownloadReleaseInfo(config.FromURL, config.PullSecret)
	} else {
		payload, err = ReadReleaseInfo(config.FromFile)
	}
	return payload, err
}

func DownloadReleaseInfo(url string, pullSecret string) (*release.ReleaseInfo, error) {
	// oc adm release info  --output json --pullspecs
	klog.InfoS("oc adm release info", "url", url)
	var cmd *exec.Cmd
	var stdout bytes.Buffer
	if pullSecret != "" {
		cmd = exec.CommandContext(context.Background(), "oc", "adm", "release", "-a", pullSecret, "info", "--output", "json", "--pullspecs", url)
	} else {
		cmd = exec.CommandContext(context.Background(), "oc", "adm", "release", "info", "--output", "json", "--pullspecs", url)
	}

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
	if err := json.Unmarshal(data, releaseInfo); err != nil {
		return nil, err
	}
	return releaseInfo, nil
}

func validateTag(ctx context.Context, tag *v1.TagReference, cfg *types.Config) *types.ScanResults {
	results := types.NewScanResults()

	image := tag.From.Name

	// skip over ignored images
	for _, ignoredImage := range cfg.FilterImages {
		if ignoredImage == image {
			klog.InfoS("Ignoring image", "image", image)
			results.Append(types.NewScanResult().SetTag(tag).Success())
			return results
		}
	}

	// pull
	if err := podman.Pull(ctx, image, cfg.InsecurePull); err != nil {
		results.Append(types.NewScanResult().SetTag(tag).SetError(err))
		return results
	}
	// mount
	mountPath, err := podman.Mount(ctx, image)
	if err != nil {
		results.Append(types.NewScanResult().SetTag(tag).SetError(err))
		return results
	}
	defer func() {
		_ = podman.Unmount(ctx, image)
	}()
	// get openshift component
	component, _ := podman.GetOpenshiftComponentFromImage(ctx, image)
	if component != nil {
		klog.InfoS("found operator", "component", component.Component, "source_location", component.SourceLocation, "maintainer_component", component.MaintainerComponent, "is_bundle", component.IsBundle)
	}
	// skip if bundle image
	if component.IsBundle {
		results.Append(types.NewScanResult().SetTag(tag).Skipped())
		return results
	}

	// does the image contain openssl
	opensslInfo := validations.ValidateOpenssl(ctx, mountPath)
	results.Append(types.NewScanResult().SetOpenssl(opensslInfo).SetTag(tag))

	// business logic for scan
	if err := filepath.WalkDir(mountPath, func(path string, file fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		innerPath := stripMountPath(mountPath, path)
		if file.IsDir() {
			if cfg.IgnoreDirWithComponent(innerPath, component) {
				return filepath.SkipDir
			}
			return nil
		}
		if !file.Type().IsRegular() {
			return nil
		}
		if cfg.IgnoreFileWithTag(innerPath, tag) || cfg.IgnoreFileWithComponent(innerPath, component) {
			return nil
		}
		klog.V(1).InfoS("scanning path", "path", path)
		res := validations.ScanBinary(ctx, component, tag, mountPath, innerPath)
		if res.Skip {
			// Do not add skipped binaries to results.
			return nil
		}
		// Check rpm.* excludes. Performed post-check because the rpm name was not known before.
		if !res.IsSuccess() && res.RPM != "" && cfg.IgnoreFileByRpm(innerPath, res.RPM) {
			return nil
		}
		if res.IsSuccess() {
			klog.V(1).InfoS("scanning success", "image", image, "path", innerPath, "status", "success")
		} else {
			status := res.Status()
			klog.InfoS("scanning "+status,
				"image", image,
				"path", innerPath,
				"error", res.Error.Error,
				"component", getComponent(res),
				"tag", res.Tag.Name,
				"rpm", res.RPM,
				"status", status)
		}
		results.Append(res)
		return nil
	}); err != nil {
		return results
	}

	return results
}

func stripMountPath(mountPath, path string) string {
	return strings.TrimPrefix(path, mountPath)
}
