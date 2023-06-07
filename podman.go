package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"k8s.io/klog/v2"
)

func podmanCreate(ctx context.Context, image string) (string, error) {
	klog.InfoS("podman: create", "image", image)
	stdout, _, err := runPodman(ctx, "create", image)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func podmanUnmount(ctx context.Context, id string) error {
	klog.InfoS("podman: unmount", "id", id)
	_, _, err := runPodman(ctx, "unmount", id)
	if err != nil {
		return err
	}
	return nil
}

func podmanMount(ctx context.Context, id string) (string, error) {
	klog.InfoS("podman: mount", "id", id)
	stdout, _, err := runPodman(ctx, "mount", id)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func podmanPull(ctx context.Context, image string) error {
	klog.InfoS("podman: pull", "image", image)
	_, _, err := runPodman(ctx, "pull", image)
	if err != nil {
		return err
	}
	return nil
}

func runPodman(ctx context.Context, args ...string) (bytes.Buffer, bytes.Buffer, error) {
	klog.InfoS("podman", "args", args)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "podman", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout, stderr, fmt.Errorf("podman error (args=%v) (stderr=%v) (error=%w)", args, stderr.String(), err)
	}
	return stdout, stderr, nil

}
