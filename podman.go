package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"k8s.io/klog/v2"
)

var alternateEntryPoints = []string{
	"/bin/sh",
	"/bin/bash",
	"/usr/bin/bash",
}

func podmanCreate(ctx context.Context, image string) (string, error) {
	stdout, _, err := runPodman(ctx, "create", image)
	if err != nil {
		for _, entryPoint := range alternateEntryPoints {
			stdout, _, err = runPodman(ctx, "create", "--entrypoint", entryPoint, image)
			if err == nil {
				break
			}
		}
		if err != nil {
			return "", err
		}
	}
	return strings.TrimSpace(stdout.String()), nil
}

func podmanUnmount(ctx context.Context, id string) error {
	_, _, err := runPodman(ctx, "unmount", id)
	if err != nil {
		return err
	}
	return nil
}

func podmanMount(ctx context.Context, id string) (string, error) {
	stdout, _, err := runPodman(ctx, "mount", id)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func podmanPull(ctx context.Context, image string, insecure bool) error {
	args := []string{"pull"}
	if insecure {
		args = append(args, "--tls-verify=false")
	}
	args = append(args, image)

	_, _, err := runPodman(ctx, args...)
	if err != nil {
		return err
	}
	return nil
}

func podmanInspect(ctx context.Context, image string, args ...string) (string, error) {
	cmdArgs := append([]string{"inspect", image}, args...)
	stdout, _, err := runPodman(ctx, cmdArgs...)
	if err != nil {
		return "", err
	}
	return stdout.String(), nil
}

func podmanContainerRm(ctx context.Context, id string) error {
	_, _, err := runPodman(ctx, "container", "rm", id)
	if err != nil {
		return err
	}
	return nil
}

func runPodman(ctx context.Context, args ...string) (bytes.Buffer, bytes.Buffer, error) {
	klog.V(1).InfoS("podman "+args[0], "args", args[1:])
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

func getOpenshiftComponentFromImage(ctx context.Context, image string) (*OpenshiftComponent, error) {
	data, err := podmanInspect(ctx, image, "--format", "{{index  .Config.Labels \"com.redhat.component\" }}|{{index  .Config.Labels \"io.openshift.build.source-location\" }}|{{index .Config.Labels \"io.openshift.maintainer.component\"}}")
	if err != nil {
		return nil, err
	}
	parts := strings.Split(data, "|")

	oc := &OpenshiftComponent{}
	oc.Component = strings.TrimSpace(parts[0])
	oc.SourceLocation = strings.TrimSpace(parts[1])
	oc.MaintainerComponent = strings.TrimSpace(parts[2])
	return oc, nil
}
