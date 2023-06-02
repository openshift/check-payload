package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	corev1 "k8s.io/api/core/v1"
)

type ArtifactPod struct {
	ApiVersion string       `json:"apiVersion"`
	Items      []corev1.Pod `json:"items"`
}

type ScanResult struct {
	Path       string
	ScanPassed bool
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

func main() {
	var help = flag.Bool("help", false, "Show help")
	var fromUrl = flag.String("url", "", "http URL to pull pods.json from")
	var fromFile = flag.String("file", defaultPodsFilename, "")

	flag.Parse()
	if *help {
		flag.Usage()
		os.Exit(0)
	}

	var apods *ArtifactPod
	var err error
	if *fromUrl != "" {
		apods, err = DownloadArtifactPods(*fromUrl)
	} else {
		apods, err = ReadArtifactPods(*fromFile)
	}
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()

	max := 5
	var runs []*ScanResults
	for i, pod := range apods.Items {
		for _, container := range pod.Spec.Containers {
			scanResults, err := validateContainer(ctx, &container)
			if err != nil {
				log.Fatal(err)
			}
			runs = append(runs, scanResults)
		}
		if i == max {
			break
		}
	}

	printResults(runs)

	if isFailed(runs) {
		fmt.Println("Test failed")
		os.Exit(1)
	}
}

func isFailed(results []*ScanResults) bool {
	for _, result := range results {
		for _, res := range result.Items {
			if !res.ScanPassed {
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
	data, err := ioutil.ReadAll(resp.Body)
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
		if mtype.Is("text/plain") || mtype.Is("text/csv") {
			return nil
		}
		results.Items = append(results.Items, scanBinary(ctx, path))
		return nil
	}); err != nil {
		log.Fatal(err)
		return nil, err
	}

	return results, nil
}

func scanBinary(ctx context.Context, path string) *ScanResult {
	result := &ScanResult{}
	result.Path = filepath.Base(path)
	result.ScanPassed = true
	return result
}

func podmanCreate(ctx context.Context, image string) (string, error) {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "podman", "create", image)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func podmanUnmount(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, "podman", "unmount", id)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func podmanMount(ctx context.Context, id string) (string, error) {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "podman", "mount", id)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func podmanPull(ctx context.Context, image string) error {
	cmd := exec.CommandContext(ctx, "podman", "pull", image)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
