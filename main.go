package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sync"
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
	PodNamespace  string
	PodName       string
	ContainerName string
	Path          string
	Error         error
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
	"vendor/github.com/golang-fips/openssl-fips/openssl._Cfunc__goboringcrypto_DLOPEN_OPENSSL",
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

	results := run(ctx, *limit, apods)
	printResults(results)

	if isFailed(results) {
		klog.Fatal("test failed")
	}
}

type Request struct {
	Pod *corev1.Pod
}

type Result struct {
	Pod     *corev1.Pod
	Results *ScanResults
}

func run(ctx context.Context, limit int64, apods *ArtifactPod) []*ScanResults {
	var runs []*ScanResults

	parallelism := 1
	tx := make(chan *Request, parallelism)
	rx := make(chan *Result, parallelism)
	var wg sync.WaitGroup

	wg.Add(parallelism)
	for i := 0; i < parallelism; i++ {
		go func() {
			scan(ctx, tx, rx)
			wg.Done()
		}()
	}

	go func() {
		for res := range rx {
			runs = append(runs, res.Results)
		}
		close(rx)
	}()

	for i, pod := range apods.Items {
		tx <- &Request{Pod: &pod}
		if limit != 0 && int64(i) == limit {
			break
		}
	}

	close(tx)
	wg.Wait()

	return runs
}

func scan(ctx context.Context, tx <-chan *Request, rx chan<- *Result) {
	for req := range tx {
		validatePod(ctx, req.Pod, rx)
	}
}

func validatePod(ctx context.Context, pod *corev1.Pod, rx chan<- *Result) {
	result := validateContainers(ctx, pod)
	rx <- &Result{Results: result}
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

func validateContainers(ctx context.Context, pod *corev1.Pod) *ScanResults {
	results := &ScanResults{}

	for _, c := range pod.Spec.Containers {
		// pull
		if err := podmanPull(ctx, c.Image); err != nil {
			results.Items = append(results.Items, NewScanResultByPod(pod).SetError(err))
			continue
		}
		// create
		createID, err := podmanCreate(ctx, c.Image)
		if err != nil {
			results.Items = append(results.Items, NewScanResultByPod(pod).SetError(err))
			continue
		}
		// mount
		mountPath, err := podmanMount(ctx, createID)
		if err != nil {
			results.Items = append(results.Items, NewScanResultByPod(pod).SetError(err))
			continue
		}
		defer func() {
			podmanUnmount(ctx, createID)
		}()

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
			res := scanBinary(ctx, pod, &c, path)
			if res.Error == nil {
				klog.InfoS("scanning success", "image", c.Image, "path", printablePath, "status", "success")
			} else {
				klog.InfoS("scanning failed", "image", c.Image, "path", printablePath, "error", res.Error, "status", "failed")
			}
			results.Items = append(results.Items, res)
			return nil
		}); err != nil {
			return results
		}
	}

	return results
}

func NewScanResult() *ScanResult {
	return &ScanResult{}
}

func NewScanResultByPod(pod *corev1.Pod) *ScanResult {
	return &ScanResult{
		PodNamespace: pod.Namespace,
		PodName:      pod.Name,
	}
}

func (r *ScanResult) Success() *ScanResult {
	r.Error = nil
	return r
}

func (r *ScanResult) SetError(err error) *ScanResult {
	r.Error = err
	return r
}

func (r *ScanResult) SetBinaryPath(path string) *ScanResult {
	r.Path = filepath.Base(path)
	return r
}
