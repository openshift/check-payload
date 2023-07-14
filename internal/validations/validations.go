package validations

import (
	"bufio"
	"bytes"
	"context"
	"debug/elf"
	"debug/gosym"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	mapset "github.com/deckarep/golang-set/v2"
	v1 "github.com/openshift/api/image/v1"

	"github.com/openshift/check-payload/internal/golang"
	"github.com/openshift/check-payload/internal/types"
)

var (
	// Compile regular expressions once during initialization.
	validateGoVersionRegexp      = regexp.MustCompile(`go(\d.\d\d)`)
	validateGoTagsRegexp         = regexp.MustCompile(`-tags=(.*)\n`)
	validateStringsOpensslRegexp = regexp.MustCompile(`libcrypto.so(\.?\d+)*`)

	ErrNotGolangExe  = errors.New("not golang executable")
	ErrNotExecutable = errors.New("not a regular executable")

	requiredGolangSymbols = []string{
		"vendor/github.com/golang-fips/openssl-fips/openssl._Cfunc__goboringcrypto_DLOPEN_OPENSSL",
		"crypto/internal/boring._Cfunc__goboringcrypto_DLOPEN_OPENSSL",
	}
)

type Baton struct {
	TopDir            string
	GoNoCrypto        bool
	GoVersion         string
	GoVersionDetailed []byte
}

type ValidationFn func(ctx context.Context, tag *v1.TagReference, path string, baton *Baton) *types.ValidationError

var validationFns = map[string][]ValidationFn{
	"go": {
		validateGoVersion,
		validateGoCgo,
		validateGoCGOInit,
		validateGoSymbols,
		validateGoStatic,
		validateGoOpenssl,
		validateGoTags,
	},
	"exe": {
		validateExe,
	},
}

func validateGoVersion(ctx context.Context, _ *v1.TagReference, path string, baton *Baton) *types.ValidationError {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", "version", "-m", path)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return types.NewValidationError(err)
	}

	matches := validateGoVersionRegexp.FindAllStringSubmatch(stdout.String(), -1)
	if len(matches) == 0 {
		return types.NewValidationError(fmt.Errorf("go: could not find compiler version in binary"))
	}
	baton.GoVersion = matches[0][1]
	baton.GoVersionDetailed = stdout.Bytes()
	return nil
}

func validateGoSymbols(_ context.Context, _ *v1.TagReference, path string, baton *Baton) *types.ValidationError {
	symtable, err := golang.ReadTable(path)
	if err != nil {
		return types.NewValidationError(fmt.Errorf("go: could not read table for %v: %w", filepath.Base(path), err))
	}
	// Skip if the golang binary is not using crypto
	if !isUsingCryptoModule(symtable) {
		baton.GoNoCrypto = true
		return nil
	}

	v, err := semver.NewVersion(baton.GoVersion)
	if err != nil {
		return types.NewValidationError(fmt.Errorf("go: error creating semver version: %w", err))
	}
	c, err := semver.NewConstraint("< 1.18")
	if err != nil {
		return types.NewValidationError(fmt.Errorf("go: error creating semver constraint: %w", err))
	}
	if c.Check(v) {
		return nil
	}

	if !golang.ExpectedSyms(requiredGolangSymbols, symtable) {
		return types.NewValidationError(fmt.Errorf("go: expected symbols not found for %v", filepath.Base(path)))
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

func validateGoCgo(_ context.Context, _ *v1.TagReference, _ string, baton *Baton) *types.ValidationError {
	v, err := semver.NewVersion(baton.GoVersion)
	if err != nil {
		return types.NewValidationError(fmt.Errorf("go: error creating semver version: %w", err))
	}
	c, err := semver.NewConstraint("<= 1.17")
	if err != nil {
		return types.NewValidationError(fmt.Errorf("go: error creating semver constraint: %w", err))
	}
	if c.Check(v) {
		return nil
	}

	if !bytes.Contains(baton.GoVersionDetailed, []byte("CGO_ENABLED=1")) {
		return types.NewValidationError(fmt.Errorf("go: binary is not CGO_ENABLED"))
	}
	return nil
}

func validateGoTags(_ context.Context, _ *v1.TagReference, _ string, baton *Baton) *types.ValidationError {
	invalidTagsSet := mapset.NewSet("no_openssl")
	expectedTagsSet := mapset.NewSet("strictfipsruntime")

	v, err := semver.NewVersion(baton.GoVersion)
	if err != nil {
		return types.NewValidationError(fmt.Errorf("go: error creating semver version: %w", err))
	}
	c, err := semver.NewConstraint("<= 1.17")
	if err != nil {
		return types.NewValidationError(fmt.Errorf("go: error creating semver constraint: %w", err))
	}
	if c.Check(v) {
		return nil
	}

	matches := validateGoTagsRegexp.FindAllSubmatch(baton.GoVersionDetailed, -1)
	if matches == nil {
		return types.NewValidationError(fmt.Errorf("go: binary has zero tags enabled (should have strictfipsruntime)")).SetWarning()
	}

	tags := strings.Split(string(matches[0][1]), ",")
	if len(tags) == 0 {
		return types.NewValidationError(fmt.Errorf("go: binary has zero tags enabled (should have strictfipsruntime)")).SetWarning()
	}

	// check for invalid tags
	binaryTags := mapset.NewSet(tags...)
	if set := binaryTags.Intersect(invalidTagsSet); set.Cardinality() > 0 {
		return types.NewValidationError(fmt.Errorf("go: binary has invalid tag %v enabled", set.ToSlice()))
	}

	// check for required tags
	if set := binaryTags.Intersect(expectedTagsSet); set.Cardinality() == 0 {
		return types.NewValidationError(fmt.Errorf("go: binary does not contain required tag(s) %v", expectedTagsSet.ToSlice())).SetWarning()
	}

	return nil
}

func validateGoStatic(ctx context.Context, _ *v1.TagReference, path string, baton *Baton) *types.ValidationError {
	// if the static golang binary does not contain crypto then skip
	if baton.GoNoCrypto {
		return nil
	}

	// check for static go
	return validateStaticGo(ctx, path)
}

func validateGoOpenssl(_ context.Context, _ *v1.TagReference, path string, baton *Baton) *types.ValidationError {
	// if there is no crypto then skip openssl test
	if baton.GoNoCrypto {
		return nil
	}
	// check for openssl strings
	return types.NewValidationError(validateStringsOpenssl(path, baton))
}

func validateGoCGOInit(_ context.Context, _ *v1.TagReference, path string, _ *Baton) *types.ValidationError {
	f, err := os.Open(path)
	if err != nil {
		return types.NewValidationError(err)
	}
	defer f.Close()

	stream := bufio.NewReader(f)

	cgoInitFound := false
	const size int = 1 * 1024 * 1024
	buf := make([]byte, size)

	for {
		n, err := stream.Read(buf)
		if err != nil && err != io.EOF {
			return types.NewValidationError(err)
		}
		if n == 0 || err == io.EOF {
			break
		}
		if bytes.Contains(buf, []byte("cgo_init")) {
			cgoInitFound = true
			break
		}
	}

	if !cgoInitFound {
		return types.NewValidationError(fmt.Errorf("x_cgo_init: not found"))
	}

	return nil
}

// scan the binary for multiple libcrypto libraries
func validateStringsOpenssl(path string, baton *Baton) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	stream := bufio.NewReader(f)

	libcryptoVersion := ""
	haveMultipleLibcrypto := false

	const size int = 1 * 1024 * 1024
	buf := make([]byte, size)

	for {
		n, err := stream.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 || err == io.EOF {
			break
		}

		matches := validateStringsOpensslRegexp.FindAllSubmatch(buf, -1)
		if len(matches) == 0 {
			continue
		}
		binaryLibcryptoVersion := string(matches[0][0])
		if binaryLibcryptoVersion == "" {
			continue
		}
		if libcryptoVersion != "" && libcryptoVersion != binaryLibcryptoVersion {
			// Have different libcrypto versions in the same binary.
			haveMultipleLibcrypto = true
		}
		libcryptoVersion = string(matches[0][0])
	}

	if libcryptoVersion == "" {
		return fmt.Errorf("openssl: did not find libcrypto library within binary")
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

func validateStaticGo(ctx context.Context, path string) *types.ValidationError {
	return isDynamicallyLinked(ctx, path)
}

func isDynamicallyLinked(ctx context.Context, path string) *types.ValidationError {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "file", "-s", path)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return types.NewValidationError(err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("dynamically linked")) {
		return types.NewValidationError(fmt.Errorf("exe: executable is statically linked"))
	}
	return nil
}

func validateExe(ctx context.Context, _ *v1.TagReference, path string, _ *Baton) *types.ValidationError {
	return isDynamicallyLinked(ctx, path)
}

func isGoExecutable(ctx context.Context, path string) error {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", "version", path)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return err
	}
	if strings.Contains(stdout.String(), ": go1.") {
		return nil
	}
	return ErrNotGolangExe
}

// isElfExe checks if path is an ELF executable (which most probably means
// it is a Linux binary).
func isElfExe(path string) (bool, error) {
	exe, err := elf.Open(path)
	if err != nil {
		var elfErr *elf.FormatError
		if errors.As(err, &elfErr) || err == io.EOF { //nolint:errorlint // See https://github.com/polyfloyd/go-errorlint/pull/45.
			// Not an ELF.
			return false, nil
		}
		// Error accessing the file.
		return false, err
	}
	defer exe.Close()
	switch exe.Type {
	case elf.ET_EXEC:
		return true, nil
	case elf.ET_DYN: // Either a binary or a shared object.
		pie, err := golang.IsPie(exe)
		if err != nil {
			return false, err
		}
		return pie, nil
	}
	// Unknown ELF file, so not a binary.
	return false, nil
}

func ScanBinary(ctx context.Context, component *types.OpenshiftComponent, tag *v1.TagReference, topDir, innerPath string) *types.ScanResult {
	allFn := validationFns["all"]
	goFn := validationFns["go"]
	exeFn := validationFns["exe"]

	baton := &Baton{TopDir: topDir}
	res := types.NewScanResult().SetComponent(component).SetTag(tag).SetPath(innerPath)

	path := filepath.Join(topDir, innerPath)

	// We are only interested in Linux binaries.
	elf, err := isElfExe(path)
	if err != nil {
		return res.SetError(err)
	}
	if !elf {
		return res.Skipped()
	}

	for _, fn := range allFn {
		if err := fn(ctx, tag, path, baton); err != nil {
			return res.SetValidationError(err)
		}
	}

	// is this a go executable
	if isGoExecutable(ctx, path) == nil {
		for _, fn := range goFn {
			if err := fn(ctx, tag, path, baton); err != nil {
				return res.SetValidationError(err)
			}
		}
	} else {
		// is a regular binary
		for _, fn := range exeFn {
			if err := fn(ctx, tag, path, baton); err != nil {
				return res.SetValidationError(err)
			}
		}
	}

	return res.Success()
}
