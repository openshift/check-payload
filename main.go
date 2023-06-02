package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
)

type ArtifactPod struct {
	ApiVersion string       `json:"apiVersion"`
	Items      []corev1.Pod `json:"items"`
}

const (
	defaultPodsFilename = "pods.json"
)

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
	for i, pod := range apods.Items {
		for _, container := range pod.Spec.Containers {
			var entryPoint string
			var err error
			if len(container.Command) == 0 {
				// need to use entrypoint of image
				entryPoint, err = fetchImageEntryPoint(ctx, container.Image)
			} else {
				entryPoint, err = resolveEntryPoint(ctx, container.Image, container.Command[0])
			}
			if err != nil {
				log.Printf("Error: %v", err)
				continue
			}
			fmt.Printf("entryPoint: %v\n", entryPoint)
			copyImageFileToHost(ctx, pod.Name, container.Name, container.Image, entryPoint)
		}
		log.Printf("Completed %d pods of %d", i+1, len(apods.Items))
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

func resolveEntryPoint(ctx context.Context, image string, command string) (string, error) {
	cmdArgs := []string{"run", "--rm", "--entrypoint", "which", image, command}
	exe := "podman"
	fmt.Printf("cmd args: %v %v\n", exe, cmdArgs)
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, exe, cmdArgs...)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func fetchImageEntryPoint(ctx context.Context, image string) (string, error) {
	//log.Printf("Pulling %v", image)
	cmd := exec.CommandContext(ctx, "podman", "pull", image)
	if err := cmd.Run(); err != nil {
		return "", err
	}
	var stdout bytes.Buffer
	cmd = exec.CommandContext(ctx, "podman", "image", "inspect", image, "--format", "{{ (index .Config.Entrypoint 0) }}")
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func copyImageFileToHost(ctx context.Context, podName string, containerName string, image string, entryPoint string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	localMountPoint := path.Join(wd, "binaries")
	if err := os.Mkdir(localMountPoint, 0770); err != nil {
		if !os.IsExist(err) {
			return err
		}
	}

	remoteMountPoint := "/binaries"
	mountArg := fmt.Sprintf("%v:%v", localMountPoint, remoteMountPoint)
	localName := path.Join(remoteMountPoint, fmt.Sprintf("%v-%v-%v", podName, containerName, path.Base(entryPoint)))

	exe := "podman"
	cmdArgs := []string{"run", "--rm", "--entrypoint", "cp", "-v", mountArg, image, entryPoint, localName}
	fmt.Printf("cp: %v", cmdArgs)
	cmd := exec.CommandContext(ctx, exe, cmdArgs...)
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
