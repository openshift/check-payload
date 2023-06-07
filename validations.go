package main

import (
	"bytes"
	"context"
	"debug/elf"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"

	v1 "github.com/openshift/api/image/v1"
)

var (
	ErrNotGolangExe  = errors.New("not golang executable")
	ErrNotExecutable = errors.New("not a regular executable")
)

type ValidationFn func(ctx context.Context, tag *v1.TagReference, path string) error

var validationFns = map[string][]ValidationFn{
	"go": {
		validateGoVersion,
		validateGoSymbols,
	},
	"exe": {
		validateExe,
	},
}

func validateGoSymbols(ctx context.Context, tag *v1.TagReference, path string) error {
	symtable, err := readTable(path)
	if err != nil {
		return fmt.Errorf("go: could not read table for %v: %v", filepath.Base(path), err)
	}
	if err := ExpectedSyms(requiredGolangSymbols, symtable); err != nil {
		return fmt.Errorf("go: expected symbols not found for %v: %v", filepath.Base(path), err)
	}
	return nil
}

func validateGoVersion(ctx context.Context, tag *v1.TagReference, path string) error {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", "version", "-m", path)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return err
	}

	// check for CGO
	if !bytes.Contains(stdout.Bytes(), []byte("CGO_ENABLED=1")) {
		return fmt.Errorf("go: binary is not CGO_ENABLED")
	}

	// verify no_openssl is not referenced
	if bytes.Contains(stdout.Bytes(), []byte("no_openssl")) {
		return fmt.Errorf("go: binary is no_openssl enabled")
	}

	// check for static go
	if err := validateStaticGo(ctx, path); err != nil {
		return err
	}

	return nil
}

func validateStaticGo(ctx context.Context, path string) error {
	return isDynamicallyLinked(ctx, path)
}

func isDynamicallyLinked(ctx context.Context, path string) error {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "file", "-s", path)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return err
	}
	if !bytes.Contains(stdout.Bytes(), []byte("dynamically linked")) {
		return fmt.Errorf("exe: executable is statically linked")
	}
	return nil
}

func validateExe(ctx context.Context, _ *v1.TagReference, path string) error {
	return isDynamicallyLinked(ctx, path)
}

func isGoExecutable(ctx context.Context, path string) error {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", "version", path)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return err
	}
	goVersionRegex := regexp.MustCompile(`.*:\s+go.*`)
	if goVersionRegex.Match(stdout.Bytes()) {
		return nil
	}
	return ErrNotGolangExe
}

func isExecutable(ctx context.Context, path string) error {
	exe, err := elf.Open(path)
	if err != nil {
		return err
	}
	defer exe.Close()
	if exe.Type != elf.ET_EXEC {
		return ErrNotExecutable
	}
	return nil
}

func scanBinary(ctx context.Context, tag *v1.TagReference, mountPath string, path string) *ScanResult {
	var allFn = validationFns["all"]
	var goFn = validationFns["go"]
	var exeFn = validationFns["exe"]

	for _, fn := range allFn {
		if err := fn(ctx, tag, path); err != nil {
			return NewScanResult().SetBinaryPath(mountPath, path).SetError(err)
		}
	}

	// is this a go executable
	if isGoExecutable(ctx, path) == nil {
		for _, fn := range goFn {
			if err := fn(ctx, tag, path); err != nil {
				return NewScanResult().SetTag(tag).SetBinaryPath(mountPath, path).SetError(err)
			}
		}
	} else if isExecutable(ctx, path) == nil {
		// is a regular binary
		for _, fn := range exeFn {
			if err := fn(ctx, tag, path); err != nil {
				return NewScanResult().SetTag(tag).SetBinaryPath(mountPath, path).SetError(err)
			}
		}
	}

	return NewScanResult().SetTag(tag).SetBinaryPath(mountPath, path).Success()
}
