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
	for _, pod := range apods.Items {
		for _, container := range pod.Spec.Containers {
			if err := validateContainer(ctx, &container); err != nil {
				log.Fatal(err)
			}
		}
		//log.Printf("Completed %d pods of %d", i+1, len(apods.Items))
		//os.Exit(1)
	}
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

func validateContainer(ctx context.Context, c *corev1.Container) error {
	// pull
	if err := podmanPull(ctx, c.Image); err != nil {
		return err
	}
	// create
	createID, err := podmanCreate(ctx, c.Image)
	if err != nil {
		return err
	}
	// mount
	mountPath, err := podmanMount(ctx, createID)
	if err != nil {
		return err
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
		if mtype.Is("text/plain") || mtype.Is("text/csv") {
			return nil
		}
		fmt.Printf("Need to scan %v (type=%v)\n", path, mtype.String())
		return nil
	}); err != nil {
		log.Fatal(err)
		return err
	}

	return nil
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
