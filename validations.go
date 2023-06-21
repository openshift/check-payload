package main

import (
	"bufio"
	"bytes"
	"context"
	"debug/elf"
	"debug/gosym"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	mapset "github.com/deckarep/golang-set/v2"
	v1 "github.com/openshift/api/image/v1"
)

var (
	ErrNotGolangExe  = errors.New("not golang executable")
	ErrNotExecutable = errors.New("not a regular executable")
)

type Baton struct {
	TopDir            string
	GoNoCrypto        bool
	GoVersion         string
	GoVersionDetailed []byte
}

type ValidationFn func(ctx context.Context, tag *v1.TagReference, path string, baton *Baton) error

var validationFns = map[string][]ValidationFn{
	"go": {
		validateGoVersion,
		validateGoCgo,
		validateGoCGOInit,
		validateGoTags,
		validateGoSymbols,
		validateGoStatic,
		validateGoOpenssl,
	},
	"exe": {
		validateExe,
	},
}

func validateGoVersion(ctx context.Context, tag *v1.TagReference, path string, baton *Baton) error {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", "version", "-m", path)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return err
	}

	golangVersionRegexp := regexp.MustCompile(`go(\d.\d\d)`)
	matches := golangVersionRegexp.FindAllStringSubmatch(stdout.String(), -1)
	if len(matches) == 0 {
		return fmt.Errorf("go: could not find compiler version in binary")
	}
	baton.GoVersion = matches[0][1]
	baton.GoVersionDetailed = stdout.Bytes()
	return nil
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

	v, err := semver.NewVersion(baton.GoVersion)
	if err != nil {
		return fmt.Errorf("go: error creating semver version: %w", err)
	}
	c, err := semver.NewConstraint("< 1.18")
	if err != nil {
		return fmt.Errorf("go: error creating semver constraint: %w", err)
	}
	if c.Check(v) {
		return nil
	}

	if err := ExpectedSyms(requiredGolangSymbols, symtable); err != nil {
		return fmt.Errorf("go: expected symbols (%v) not found for %v: %v", requiredGolangSymbols, filepath.Base(path), err)
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

	stdout.Reset()

	cmd = exec.CommandContext(ctx, "file", path)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return err
	}

	if bytes.Contains(stdout.Bytes(), []byte("ELF")) {
		return nil
	}

	return fmt.Errorf("go: not a linux binary")
}

func validateGoCgo(ctx context.Context, tag *v1.TagReference, path string, baton *Baton) error {
	v, err := semver.NewVersion(baton.GoVersion)
	if err != nil {
		return fmt.Errorf("go: error creating semver version: %w", err)
	}
	c, err := semver.NewConstraint("<= 1.17")
	if err != nil {
		return fmt.Errorf("go: error creating semver constraint: %w", err)
	}
	if c.Check(v) {
		return nil
	}

	if !bytes.Contains(baton.GoVersionDetailed, []byte("CGO_ENABLED=1")) {
		return fmt.Errorf("go: binary is not CGO_ENABLED")
	}
	return nil
}

func validateGoTags(ctx context.Context, tag *v1.TagReference, path string, baton *Baton) error {
	invalidTagsSet := mapset.NewSet[string]("no_openssl")
	expectedTagsSet := mapset.NewSet[string]("strictfipsruntime")

	v, err := semver.NewVersion(baton.GoVersion)
	if err != nil {
		return fmt.Errorf("go: error creating semver version: %w", err)
	}
	c, err := semver.NewConstraint("<= 1.17")
	if err != nil {
		return fmt.Errorf("go: error creating semver constraint: %w", err)
	}
	if c.Check(v) {
		return nil
	}

	golangTagsRegexp := regexp.MustCompile(`-tags=(.*)\n`)
	matches := golangTagsRegexp.FindAllSubmatch(baton.GoVersionDetailed, -1)
	if matches == nil {
		return nil
	}

	tags := strings.Split(string(matches[0][1]), ",")
	if len(tags) == 0 {
		return nil
	}

	// check for invalid tags
	binaryTags := mapset.NewSet[string](tags...)
	if set := binaryTags.Intersect(invalidTagsSet); set.Cardinality() > 0 {
		return fmt.Errorf("go: binary has invalid tag %v enabled", set.ToSlice())
	}

	// check for required tags
	if set := binaryTags.Intersect(expectedTagsSet); set.Cardinality() == 0 {
		return fmt.Errorf("go: binary does not contain required tag(s) %v", expectedTagsSet.ToSlice())
	}

	return nil
}

func validateGoStatic(ctx context.Context, tag *v1.TagReference, path string, baton *Baton) error {
	// if the static golang binary does not contain crypto then skip
	if baton.GoNoCrypto {
		return nil
	}

	// check for static go
	return validateStaticGo(ctx, path)
}

func validateGoOpenssl(ctx context.Context, tag *v1.TagReference, path string, baton *Baton) error {
	// check for openssl strings
	return validateStringsOpenssl(ctx, path, baton)
}

func validateGoCGOInit(ctx context.Context, tag *v1.TagReference, path string, baton *Baton) error {
	cmd := exec.CommandContext(ctx, "strings", path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	cgoInitFound := false

	scanner := bufio.NewScanner(stdout)
	const cap int = 1 * 1024 * 1024
	buf := make([]byte, cap)
	scanner.Buffer(buf, cap)

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "cgo_init") {
			cgoInitFound = true
			cmd.Cancel()
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		if !strings.Contains(err.Error(), "signal: killed") {
			return err
		}
	}

	if !cgoInitFound {
		return fmt.Errorf("x_cgo_init: not found")
	}

	return nil
}

// scan the binary for multiple libcrypto libraries
func validateStringsOpenssl(ctx context.Context, path string, baton *Baton) error {
	cmd := exec.CommandContext(ctx, "strings", path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	libcryptoVersion := ""
	cryptoRegexp := regexp.MustCompile(`libcrypto.so(\.?\d+)*`)
	haveMultipleLibcrypto := false

	scanner := bufio.NewScanner(stdout)
	const cap int = 1 * 1024 * 1024
	buf := make([]byte, cap)
	scanner.Buffer(buf, cap)

	for scanner.Scan() {
		text := scanner.Text()
		if strings.Contains(text, "libcrypto") {
			matches := cryptoRegexp.FindAllStringSubmatch(text, -1)
			if len(matches) == 0 {
				continue
			}
			if libcryptoVersion != "" && libcryptoVersion != matches[0][0] {
				// Have different libcrypto versions in the same binary.
				haveMultipleLibcrypto = true
			}
			libcryptoVersion = matches[0][0]
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	if libcryptoVersion == "" {
		return fmt.Errorf("openssl: did not find libcrypto libraries")
	}

	if haveMultipleLibcrypto {
		return errors.New("openssl: found multiple different libcrypto versions")
	}

	// check for openssl library within container image
	opensslInnerPath := filepath.Join(baton.TopDir, "usr", "lib64", libcryptoVersion)
	if _, err := os.Lstat(opensslInnerPath); err != nil {
		return fmt.Errorf("could not find dependent openssl version %v within container image", libcryptoVersion)
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

func scanBinary(ctx context.Context, operator string, tag *v1.TagReference, topDir, innerPath string) *ScanResult {
	allFn := validationFns["all"]
	goFn := validationFns["go"]
	exeFn := validationFns["exe"]

	baton := &Baton{TopDir: topDir}
	res := NewScanResult().SetOperator(operator).SetTag(tag).SetPath(innerPath)

	path := filepath.Join(topDir, innerPath)
	for _, fn := range allFn {
		if err := fn(ctx, tag, path, baton); err != nil {
			return res.SetError(err)
		}
	}

	// is this a go executable
	if isGoExecutable(ctx, path) == nil {
		for _, fn := range goFn {
			// make sure the binary is linux
			if err := validateGoLinux(ctx, tag, path); err != nil {
				// we only scan linux binaries so this is successful
				return res.Success()
			}
			if err := fn(ctx, tag, path, baton); err != nil {
				return res.SetError(err)
			}
		}
	} else if isExecutable(ctx, path) == nil {
		// is a regular binary
		for _, fn := range exeFn {
			if err := fn(ctx, tag, path, baton); err != nil {
				return res.SetError(err)
			}
		}
	}

	return res.Success()
}
