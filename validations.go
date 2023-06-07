package main

import (
	"bufio"
	"bytes"
	"context"
	"debug/elf"
	"debug/gosym"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	v1 "github.com/openshift/api/image/v1"
)

var (
	ErrNotGolangExe  = errors.New("not golang executable")
	ErrNotExecutable = errors.New("not a regular executable")
)

type Baton struct {
	GoNoCrypto bool
}

type ValidationFn func(ctx context.Context, tag *v1.TagReference, path string, baton *Baton) error

var validationFns = map[string][]ValidationFn{
	"go": {
		validateGoCgo,
		validateGoNoOpenssl,
		validateGoSymbols,
		validateGoStatic,
		validateGoOpenssl,
	},
	"exe": {
		validateExe,
	},
}

func validateGoSymbols(ctx context.Context, tag *v1.TagReference, path string, baton *Baton) error {
	symtable, err := readTable(path)
	if err != nil {
		return fmt.Errorf("go: could not read table for %v: %v", filepath.Base(path), err)
	}
	// Skip if the golang binary is not using crypto
	if !isUsingCryptoModule(symtable) {
		baton.GoNoCrypto = true
		return nil
	}
	if err := ExpectedSyms(requiredGolangSymbols, symtable); err != nil {
		return fmt.Errorf("go: expected symbols not found for %v: %v", filepath.Base(path), err)
	}
	return nil
}

func isUsingCryptoModule(symtable *gosym.Table) bool {
	for _, fn := range symtable.Funcs {
		if strings.Contains(fn.Name, "crypto") {
			return true
		}
	}
	return false
}

func validateGoLinux(ctx context.Context, tag *v1.TagReference, path string) error {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", "version", "-m", path)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return err
	}

	if bytes.Contains(stdout.Bytes(), []byte("GOOS=linux")) {
		return nil
	}

	return fmt.Errorf("go: not a linux binary")
}

func validateGoCgo(ctx context.Context, tag *v1.TagReference, path string, baton *Baton) error {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", "version", "-m", path)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return err
	}

	skipCGOEnabledCheck := bytes.Contains(stdout.Bytes(), []byte("go1.16"))

	// check for CGO
	if !skipCGOEnabledCheck && !bytes.Contains(stdout.Bytes(), []byte("CGO_ENABLED=1")) {
		return fmt.Errorf("go: binary is not CGO_ENABLED")
	}

	return nil
}

func validateGoNoOpenssl(ctx context.Context, tag *v1.TagReference, path string, baton *Baton) error {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", "version", "-m", path)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return err
	}

	// verify no_openssl is not referenced
	if bytes.Contains(stdout.Bytes(), []byte("no_openssl")) {
		return fmt.Errorf("go: binary is no_openssl enabled")
	}

	return nil
}

func validateGoStatic(ctx context.Context, tag *v1.TagReference, path string, baton *Baton) error {
	// if the static golang binary does not contain crypto then skip
	if baton.GoNoCrypto {
		return nil
	}

	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", "version", "-m", path)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return err
	}

	// check for static go
	return validateStaticGo(ctx, path)
}

func validateGoOpenssl(ctx context.Context, tag *v1.TagReference, path string, baton *Baton) error {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", "version", "-m", path)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return err
	}

	// check for openssl strings
	return validateStringsOpenssl(ctx, path)
}

// scan the binary for multiple libcrypto libraries
func validateStringsOpenssl(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "strings", path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	sslLibraryCount := 0
	var invalidPaths []string

	scanner := bufio.NewScanner(stdout)
	const cap int = 1 * 1024 * 1024
	buf := make([]byte, cap)
	scanner.Buffer(buf, cap)

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "libcrypto") {
			sslLibraryCount++
			invalidPaths = append(invalidPaths, scanner.Text())
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	// should only find 1 libcrypto library linked in, there can be multiples so skip over the same ones
	if sslLibraryCount > 1 && !isSliceEqual(invalidPaths, invalidPaths[0]) {
		return fmt.Errorf("openssl: found %v libcrypto libraries (paths=%v)", sslLibraryCount, invalidPaths)
	}

	return nil
}

func isSliceEqual(list []string, comparison string) bool {
	for _, a := range list {
		if a != comparison {
			return false
		}
	}
	return true
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

func validateExe(ctx context.Context, _ *v1.TagReference, path string, baton *Baton) error {
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

	baton := &Baton{}

	for _, fn := range allFn {
		if err := fn(ctx, tag, path, baton); err != nil {
			return NewScanResult().SetBinaryPath(mountPath, path).SetError(err)
		}
	}

	// is this a go executable
	if isGoExecutable(ctx, path) == nil {
		for _, fn := range goFn {
			// make sure the binary is linux
			if err := validateGoLinux(ctx, tag, path); err != nil {
				// we only scan linux binaries so this is successful
				return NewScanResult().SetTag(tag).SetBinaryPath(mountPath, path).Success()
			}
			if err := fn(ctx, tag, path, baton); err != nil {
				return NewScanResult().SetTag(tag).SetBinaryPath(mountPath, path).SetError(err)
			}
		}
	} else if isExecutable(ctx, path) == nil {
		// is a regular binary
		for _, fn := range exeFn {
			if err := fn(ctx, tag, path, baton); err != nil {
				return NewScanResult().SetTag(tag).SetBinaryPath(mountPath, path).SetError(err)
			}
		}
	}

	return NewScanResult().SetTag(tag).SetBinaryPath(mountPath, path).Success()
}
