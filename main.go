package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"k8s.io/klog/v2"

	"github.com/gabriel-vasile/mimetype"
	corev1 "k8s.io/api/core/v1"
)

type ArtifactPod struct {
	ApiVersion string       `json:"apiVersion"`
	Items      []corev1.Pod `json:"items"`
}

type ScanResult struct {
	Path  string
	Error error
}

type ScanResults struct {
	Items []*ScanResult
}

const (
	defaultPodsFilename = "pods.json"
)

var ignoredMimes = []string{
	"application/gzip",
	"application/json",
	"application/octet-stream",
	"application/tzif",
	"application/vnd.sqlite3",
	"application/x-sharedlib",
	"application/zip",
	"text/csv",
	"text/html",
	"text/plain",
	"text/tab-separated-values",
	"text/xml",
	"text/x-python",
}

var requiredGolangSymbols = []string{
	"vendor/github.com/golang-fips/openssl-fips/openssl.init",
}

func main() {
	var help = flag.Bool("help", false, "show help")
	var fromUrl = flag.String("url", "", "http URL to pull pods.json from")
	var fromFile = flag.String("file", defaultPodsFilename, "")
	var limit = flag.Int64("limit", 0, "limit the number of pods scanned")
	var timeLimit = flag.Duration("time-limit", 1*time.Hour, "limit running time")

	flag.Parse()
	if *help {
		flag.Usage()
		os.Exit(0)
	}

	klog.InitFlags(nil)

	apods, err := getPods(fromUrl, fromFile)
	if err != nil {
		klog.Fatalf("could not get pods: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeLimit)
	defer cancel()

	runs := run(ctx, *limit, apods)
	printResults(runs)

	if isFailed(runs) {
		klog.Fatal("test failed")
	}
}

func run(ctx context.Context, limit int64, apods *ArtifactPod) []*ScanResults {
	var runs []*ScanResults
	for i, pod := range apods.Items {
		for _, container := range pod.Spec.Containers {
			scanResults, err := validateContainer(ctx, &container)
			if err != nil {
				klog.Errorf("error with pod %v/%v %v\n", pod.Namespace, pod.Name, err)
			}
			runs = append(runs, scanResults)
		}
		if limit != 0 && int64(i) == limit {
			break
		}
	}
	return runs
}

func getPods(fromUrl *string, fromFile *string) (*ArtifactPod, error) {
	var apods *ArtifactPod
	var err error
	if *fromUrl != "" {
		apods, err = DownloadArtifactPods(*fromUrl)
	} else {
		apods, err = ReadArtifactPods(*fromFile)
	}
	return apods, err
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

func DownloadArtifactPods(url string) (*ArtifactPod, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	apod := &ArtifactPod{}
	if err := json.Unmarshal([]byte(data), &apod); err != nil {
		return nil, err
	}
	return apod, nil
}

func ReadArtifactPods(filename string) (*ArtifactPod, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	apod := &ArtifactPod{}
	if err := json.Unmarshal([]byte(data), &apod); err != nil {
		return nil, err
	}
	return apod, nil
}

func validateContainer(ctx context.Context, c *corev1.Container) (*ScanResults, error) {
	// pull
	if err := podmanPull(ctx, c.Image); err != nil {
		return nil, err
	}
	// create
	createID, err := podmanCreate(ctx, c.Image)
	if err != nil {
		return nil, err
	}
	// mount
	mountPath, err := podmanMount(ctx, createID)
	if err != nil {
		return nil, err
	}
	defer func() {
		podmanUnmount(ctx, createID)
	}()

	results := &ScanResults{}

	// business logic for scan
	if err := filepath.WalkDir(mountPath, func(path string, file fs.DirEntry, err error) error {
		if err != nil {
			return err
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
		printablePath := filepath.Base(path)
		klog.InfoS("scanning image", "image", c.Image, "path", printablePath)
		res := scanBinary(ctx, path)
		if res.Error == nil {
			klog.InfoS("scanning success", "image", c.Image, "path", printablePath, "status", "success")
		} else {
			klog.InfoS("scanning failed", "image", c.Image, "path", printablePath, "error", res.Error, "status", "failed")
		}
		results.Items = append(results.Items, res)
		return nil
	}); err != nil {
		return nil, err
	}

	return results, nil
}

type ValidationFn func(ctx context.Context, path string) error

var validationFns = map[string][]ValidationFn{
	"go":  {validateGoSymbols},
	"all": {},
}

func validateGoSymbols(ctx context.Context, path string) error {
	symtable, err := readTable(path)
	if err != nil {
		return fmt.Errorf("expected symbols not found for %v: %v", path, err)
	}
	if err := ExpectedSyms(requiredGolangSymbols, symtable); err != nil {
		return fmt.Errorf("expected symbols not found for %v: %v", path, err)
	}
	return nil
}

func NewScanResult(path string, err error) *ScanResult {
	return &ScanResult{
		Path:  path,
		Error: err,
	}
}

func scanBinary(ctx context.Context, path string) *ScanResult {
	var allFn = validationFns["all"]
	var goFn = validationFns["go"]

	for _, fn := range allFn {
		if err := fn(ctx, path); err != nil {
			return NewScanResult(path, err)
		}
	}

	for _, fn := range goFn {
		if err := fn(ctx, path); err != nil {
			return NewScanResult(path, err)
		}
	}

	return NewScanResult(path, nil)
}
