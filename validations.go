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
		validateGoSymbols,
		validateGoVersion,
	},
	"exe": {
		validateExe,
	},
}

func validateGoSymbols(ctx context.Context, tag *v1.TagReference, path string) error {
	symtable, err := readTable(path)
	if err != nil {
		return fmt.Errorf("go: expected symbols not found for %v: %v", filepath.Base(path), err)
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

	// check for static go
	if err := validateStaticGo(ctx, path); err != nil {
		return err
	}

	return nil
}

func validateStaticGo(ctx context.Context, path string) error {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "file", "-s", path)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return err
	}
	if !bytes.Contains(stdout.Bytes(), []byte("dynamically linked")) {
		return fmt.Errorf("go: executable is statically linked")
	}
	return nil
}

func validateExe(ctx context.Context, tag *v1.TagReference, path string) error {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "readelf", "-d", path)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return err
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Shared library: [libdl")) {
		return fmt.Errorf("exe: binary is not dynamic executable with libdl")
	}
	return nil
}

func isGoExecutable(ctx context.Context, path string) error {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", "version", path)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return err
	}
	goVersionRegex := regexp.MustCompile(`.*:\s+go.*`)
	if isGo := goVersionRegex.Match(stdout.Bytes()); isGo {
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

func scanBinary(ctx context.Context, tag *v1.TagReference, path string) *ScanResult {
	var allFn = validationFns["all"]
	var goFn = validationFns["go"]
	var exeFn = validationFns["exe"]

	for _, fn := range allFn {
		if err := fn(ctx, tag, path); err != nil {
			return NewScanResult().SetBinaryPath(path).SetError(err)
		}
	}

	// is this a go executable
	if isGoExecutable(ctx, path) == nil {
		for _, fn := range goFn {
			if err := fn(ctx, tag, path); err != nil {
				return NewScanResult().SetTag(tag).SetBinaryPath(path).SetError(err)
			}
		}
	} else if isExecutable(ctx, path) == nil {
		// is a regular binary
		for _, fn := range exeFn {
			if err := fn(ctx, tag, path); err != nil {
				return NewScanResult().SetTag(tag).SetBinaryPath(path).SetError(err)
			}
		}
	}

	return NewScanResult().SetTag(tag).SetBinaryPath(path).Success()
}
