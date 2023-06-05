package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
)

type ValidationFn func(ctx context.Context, container *corev1.Container, path string) error

var validationFns = map[string][]ValidationFn{
	"go": {
		validateGoSymbols,
		validateGoVersion,
	},
	"all": {},
}

func validateGoSymbols(ctx context.Context, container *corev1.Container, path string) error {
	symtable, err := readTable(path)
	if err != nil {
		return fmt.Errorf("expected symbols not found for %v: %v", filepath.Base(path), err)
	}
	if err := ExpectedSyms(requiredGolangSymbols, symtable); err != nil {
		return fmt.Errorf("expected symbols not found for %v: %v", filepath.Base(path), err)
	}
	return nil
}

func validateGoVersion(ctx context.Context, container *corev1.Container, path string) error {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", "version", "-m", path)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return err
	}

	if !bytes.Contains(stdout.Bytes(), []byte("CGO_ENABLED")) || !bytes.Contains(stdout.Bytes(), []byte("ldflags")) {
		return fmt.Errorf("binary is not CGO_ENABLED or static with ldflags")
	}

	return nil
}

func scanBinary(ctx context.Context, pod *corev1.Pod, container *corev1.Container, path string) *ScanResult {
	var allFn = validationFns["all"]
	var goFn = validationFns["go"]

	for _, fn := range allFn {
		if err := fn(ctx, container, path); err != nil {
			return NewScanResult().SetBinaryPath(path).SetError(err)
		}
	}

	for _, fn := range goFn {
		if err := fn(ctx, container, path); err != nil {
			return NewScanResultByPod(pod).SetBinaryPath(path).SetError(err)
		}
	}

	return NewScanResultByPod(pod).SetBinaryPath(path).Success()
}
