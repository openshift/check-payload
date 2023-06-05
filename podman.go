package main

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"k8s.io/klog/v2"
)

func podmanCreate(ctx context.Context, image string) (string, error) {
	klog.Infof("podman: create %v", image)
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "podman", "create", image)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func podmanUnmount(ctx context.Context, id string) error {
	klog.Infof("podman: unmount %v", id)
	cmd := exec.CommandContext(ctx, "podman", "unmount", id)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func podmanMount(ctx context.Context, id string) (string, error) {
	klog.Infof("podman: mount%v", id)
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "podman", "mount", id)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func podmanPull(ctx context.Context, image string) error {
	klog.Infof("podman: pull %v", image)
	cmd := exec.CommandContext(ctx, "podman", "pull", image)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
