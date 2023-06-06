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
)

const (
	defaultPayloadFilename = "payload.json"
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
	"crypto/internal/boring._Cfunc__goboringcrypto_DLOPEN_OPENSSL",
}

func main() {
	var help = flag.Bool("help", false, "show help")
	var fromUrl = flag.String("url", "", "http URL to pull payload from")
	var fromFile = flag.String("file", defaultPayloadFilename, "json file for payload")
	var limit = flag.Int("limit", 0, "limit the number of pods scanned")
	var timeLimit = flag.Duration("time-limit", 1*time.Hour, "limit running time")
	var parallelism = flag.Int("parallelism", 5, "how many pods to check at once")
	var outputFormat = flag.String("output-format", "table", "output format (table, csv, markdown, html)")
	var outputFile = flag.String("output-file", "", "write report to this file")
	var components = flag.String("components", "", "scan a specific set of components")

	flag.Parse()
	if *help {
		flag.Usage()
		os.Exit(0)
	}

	config := Config{
		FromURL:      *fromUrl,
		FromFile:     *fromFile,
		Limit:        *limit,
		TimeLimit:    *timeLimit,
		Parallelism:  *parallelism,
		OutputFormat: *outputFormat,
		OutputFile:   *outputFile,
	}

	if *components != "" {
		config.Components = strings.Split(*components, ",")
	}

	klog.InitFlags(nil)

	apods, err := GetPods(&config)
	if err != nil {
		klog.Fatalf("could not get pods: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeLimit)
	defer cancel()

	results := run(ctx, &config, apods)
	err = printResults(&config, results)

	if err != nil || isFailed(results) {
		os.Exit(1)
	}
}

type Request struct {
	Tag *v1.TagReference
}

type Result struct {
	Tag     *v1.TagReference
	Results *ScanResults
}

func run(ctx context.Context, config *Config, payload *release.ReleaseInfo) []*ScanResults {
	var runs []*ScanResults

	parallelism := config.Parallelism
	limit := config.Limit

	tx := make(chan *Request, parallelism)
	rx := make(chan *Result, parallelism)
	var wg sync.WaitGroup

	wg.Add(config.Parallelism)
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
		if len(config.Components) > 0 && !contains(config.Components, tag.Name) {
			continue
		}
		tag := tag
		tx <- &Request{Tag: &tag}
		if limit != 0 && int(i) == limit-1 {
			break
		}
	}

	close(tx)
	wg.Wait()

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

func GetPods(config *Config) (*release.ReleaseInfo, error) {
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
	results := &ScanResults{}

	image := tag.From.Name

	// pull
	if err := podmanPull(ctx, image); err != nil {
		results.Items = append(results.Items, NewScanResult().SetTag(tag).SetError(err))
		return results
	}
	// create
	createID, err := podmanCreate(ctx, image)
	if err != nil {
		results.Items = append(results.Items, NewScanResult().SetTag(tag).SetError(err))
		return results
	}
	// mount
	mountPath, err := podmanMount(ctx, createID)
	if err != nil {
		results.Items = append(results.Items, NewScanResult().SetTag(tag).SetError(err))
		return results
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
		klog.InfoS("scanning tag", "tag", tag)
		res := scanBinary(ctx, tag, path)
		if res.Error == nil {
			klog.InfoS("scanning success", "image", image, "path", printablePath, "status", "success")
		} else {
			klog.InfoS("scanning failed", "image", image, "path", printablePath, "error", res.Error, "status", "failed")
		}
		results.Items = append(results.Items, res)
		return nil
	}); err != nil {
		return results
	}

	return results
}
