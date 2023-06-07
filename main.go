package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/gabriel-vasile/mimetype"
	v1 "github.com/openshift/api/image/v1"
	"github.com/openshift/oc/pkg/cli/admin/release"
	corev1 "k8s.io/api/core/v1"
)

const (
	defaultPayloadFilename = "payload.json"
)

var applicationDeps = []string{
	"file",
	"go",
	"nm",
	"oc",
	"podman",
	"readelf",
	"strings",
}

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
	"crypto/internal/boring._Cfunc__goboringcrypto_DLOPEN_OPENSSL",
}

func main() {
	var operatorImage = flag.String("operator-image", "", "only run scan on operator image")
	var components = flag.String("components", "", "scan a specific set of components")
	var fromFile = flag.String("file", defaultPayloadFilename, "json file for payload")
	var fromUrl = flag.String("url", "", "http URL to pull payload from")
	var help = flag.Bool("help", false, "show help")
	var limit = flag.Int("limit", 0, "limit the number of pods scanned")
	var outputFile = flag.String("output-file", "", "write report to this file")
	var outputFormat = flag.String("output-format", "table", "output format (table, csv, markdown, html)")
	var parallelism = flag.Int("parallelism", 5, "how many pods to check at once")
	var timeLimit = flag.Duration("time-limit", 1*time.Hour, "limit running time")
	var verbose = flag.Bool("verbose", false, "verbose")

	flag.Parse()
	if *help {
		flag.Usage()
		os.Exit(0)
	}

	config := Config{
		FromFile:      *fromFile,
		FromURL:       *fromUrl,
		Limit:         *limit,
		OperatorImage: *operatorImage,
		OutputFile:    *outputFile,
		OutputFormat:  *outputFormat,
		Parallelism:   *parallelism,
		TimeLimit:     *timeLimit,
		Verbose:       *verbose,
	}

	if *components != "" {
		config.Components = strings.Split(*components, ",")
	}

	klog.InitFlags(nil)

	validateApplicationDependencies()

	ctx, cancel := context.WithTimeout(context.Background(), *timeLimit)
	defer cancel()

	results := run(ctx, &config)
	err := printResults(&config, results)
	if err != nil || isFailed(results) {
		os.Exit(1)
	}
}

func validateApplicationDependencies() {
	for _, app := range applicationDeps {
		if _, err := exec.LookPath(app); err != nil {
			klog.Fatal("dependency application not found: %v", app)
		}
	}
}

type Request struct {
	Tag *v1.TagReference
}

type Result struct {
	Tag     *v1.TagReference
	Results *ScanResults
}

func run(ctx context.Context, cfg *Config) []*ScanResults {
	if cfg.OperatorImage != "" {
		return runOperatorScan(ctx, cfg)
	}
	return runPayloadScan(ctx, cfg)
}

func runOperatorScan(ctx context.Context, cfg *Config) []*ScanResults {
	tag := &v1.TagReference{
		From: &corev1.ObjectReference{
			Name: cfg.OperatorImage,
		},
	}

	results := validateTag(ctx, tag)

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
			scan(ctx, tx, rx)
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
		if limit != 0 && int(i) == limit-1 {
			break
		}
	}

	close(tx)
	wgThreads.Wait()
	close(rx)
	wgRx.Wait()

	return runs
}

func scan(ctx context.Context, tx <-chan *Request, rx chan<- *Result) {
	for req := range tx {
		ValidateTag(ctx, req.Tag, rx)
	}
}

func ValidateTag(ctx context.Context, tag *v1.TagReference, rx chan<- *Result) {
	result := validateTag(ctx, tag)
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

func validateTag(ctx context.Context, tag *v1.TagReference) *ScanResults {
	results := NewScanResults()

	image := tag.From.Name

	// pull
	if err := podmanPull(ctx, image); err != nil {
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
		printablePath := stripMountPath(mountPath, path)
		klog.InfoS("scanning tag", "tag", tag)
		res := scanBinary(ctx, tag, mountPath, path)
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

func stripMountPath(mountPath, path string) string {
	return strings.TrimPrefix(path, mountPath)
}
