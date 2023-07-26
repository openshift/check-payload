package validations

import (
	"bufio"
	"bytes"
	"context"
	"debug/buildinfo"
	"debug/elf"
	"debug/gosym"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"k8s.io/klog/v2"

	"github.com/openshift/check-payload/internal/golang"
	"github.com/openshift/check-payload/internal/rpm"
	"github.com/openshift/check-payload/internal/types"
)

var (
	// Compile regular expressions once during initialization.
	validateStringsOpensslRegexp = regexp.MustCompile(`libcrypto.so(\.?\d+)*`)

	requiredGolangSymbols = []string{
		"vendor/github.com/golang-fips/openssl-fips/openssl._Cfunc__goboringcrypto_DLOPEN_OPENSSL",
		"crypto/internal/boring._Cfunc__goboringcrypto_DLOPEN_OPENSSL",
	}

	goLessThan118 = newSemverConstraint("< 1.18")
)

type Baton struct {
	TopDir      string
	Static      bool
	GoNoCrypto  bool
	GoVersion   *semver.Version
	GoBuildInfo *buildinfo.BuildInfo
}

type ValidationFn func(ctx context.Context, path string, baton *Baton) *types.ValidationError

var validationFns = map[string][]ValidationFn{
	"go": {
		validateGoCgo,
		validateGoCGOInit,
		validateGoSymbols,
		validateGoStatic,
		validateGoOpenssl,
		validateGoTags,
	},
	"exe": {
		validateNotStatic,
	},
}

func validateGoSymbols(_ context.Context, path string, baton *Baton) *types.ValidationError {
	symtable, err := golang.ReadTable(path, baton.GoBuildInfo)
	if err != nil {
		return types.NewValidationError(fmt.Errorf("go: could not read table for %v: %w", filepath.Base(path), err))
	}
	// Skip if the golang binary is not using crypto
	if !isUsingCryptoModule(symtable) {
		baton.GoNoCrypto = true
		return nil
	}

	if goLessThan118.Check(baton.GoVersion) {
		return nil
	}

	if !golang.ExpectedSyms(requiredGolangSymbols, symtable) {
		return types.NewValidationError(types.ErrGoMissingSymbols)
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

func validateGoCgo(_ context.Context, _ string, baton *Baton) *types.ValidationError {
	if goLessThan118.Check(baton.GoVersion) {
		return nil
	}
	for _, bs := range baton.GoBuildInfo.Settings {
		if bs.Key == "CGO_ENABLED" && bs.Value == "1" {
			return nil
		}
	}
	return types.NewValidationError(types.ErrGoNotCgoEnabled)
}

func validateGoTags(_ context.Context, _ string, baton *Baton) *types.ValidationError {
	badTags := []string{"no_openssl"}
	goodTags := []string{"strictfipsruntime"}

	if goLessThan118.Check(baton.GoVersion) {
		return nil
	}

	tags := "x"
	for _, bs := range baton.GoBuildInfo.Settings {
		if bs.Key == "-tags" {
			tags = "," + bs.Value + ","
			break
		}
	}
	if tags == "x" {
		return types.NewValidationError(types.ErrGoNoTags).SetWarning()
	}

	// Check for invalid tags.
	for _, tag := range badTags {
		if strings.Contains(tags, ","+tag+",") {
			return types.NewValidationError(fmt.Errorf("%w: %v", types.ErrGoInvalidTag, tag))
		}
	}

	// Check for required tags.
	for _, tag := range goodTags {
		if !strings.Contains(tags, ","+tag+",") {
			return types.NewValidationError(fmt.Errorf("%w: %v", types.ErrGoMissingTag, tag)).SetWarning()
		}
	}

	return nil
}

func validateGoStatic(ctx context.Context, path string, baton *Baton) *types.ValidationError {
	// if the static golang binary does not contain crypto then skip
	if baton.GoNoCrypto {
		return nil
	}
	return validateNotStatic(ctx, path, baton)
}

func validateGoOpenssl(_ context.Context, path string, baton *Baton) *types.ValidationError {
	// if there is no crypto then skip openssl test
	if baton.GoNoCrypto {
		return nil
	}
	// check for openssl strings
	return types.NewValidationError(validateStringsOpenssl(path, baton))
}

func validateGoCGOInit(_ context.Context, path string, _ *Baton) *types.ValidationError {
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
		return types.NewValidationError(types.ErrGoNoCgoInit)
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

		binaryLibcryptoVersion := string(validateStringsOpensslRegexp.Find(buf))
		if binaryLibcryptoVersion == "" {
			continue
		}
		if libcryptoVersion != "" && libcryptoVersion != binaryLibcryptoVersion {
			// Have different libcrypto versions in the same binary.
			haveMultipleLibcrypto = true
		}
		libcryptoVersion = binaryLibcryptoVersion
	}

	if libcryptoVersion == "" {
		return types.ErrLibcryptoMissing
	}

	if haveMultipleLibcrypto {
		return types.ErrLibcryptoMany
	}

	// check for openssl library within container image
	opensslInnerPath := filepath.Join(baton.TopDir, "usr", "lib64", libcryptoVersion)
	if _, err := os.Lstat(opensslInnerPath); err != nil {
		return fmt.Errorf("%w: %v", types.ErrLibcryptoSoMissing, libcryptoVersion)
	}

	return nil
}

func validateNotStatic(_ context.Context, _ string, baton *Baton) *types.ValidationError {
	if !baton.Static {
		return nil
	}
	return types.NewValidationError(types.ErrNotDynLinked)
}

func isGoExecutable(path string, baton *Baton) (bool, error) {
	bi, err := buildinfo.ReadFile(path)
	if err != nil {
		// We do not return an error from buildinfo.ReadFile here because
		// it can either be about non-readable binary or a non-binary, and
		// it is somewhat complicated to distinguish between the two.
		return false, nil
	}

	baton.GoBuildInfo = bi
	// Remove the go prefix.
	ver := strings.TrimPrefix(bi.GoVersion, "go")
	// Remove a potential suffix after a space.
	if i := strings.IndexByte(ver, ' '); i != -1 {
		ver = ver[:i]
	}
	baton.GoVersion, err = semver.NewVersion(ver)
	if err != nil {
		return false, err
	}

	return true, nil
}

// isStatic tells if exe is a static binary.
func isStatic(exe *elf.File) bool {
	for _, p := range exe.Progs {
		// Static binaries do not have a PT_INTERP program.
		if p.Type == elf.PT_INTERP {
			return false
		}
	}
	return true
}

// isElfExe checks if path is an ELF executable (which most probably means
// it is a Linux binary). For ELF executables, it also checks if the binary
// is dynamic or static, and sets baton.Dynamic accordingly.
func isElfExe(path string, baton *Baton) (bool, error) {
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
		baton.Static = isStatic(exe)
		return true, nil
	case elf.ET_DYN: // Either a binary or a shared object.
		pie, err := golang.IsPie(exe)
		if err != nil || !pie {
			return false, err
		}
		baton.Static = isStatic(exe)
		return true, nil
	}
	// Unknown ELF file, so not a binary.
	return false, nil
}

func ScanBinary(ctx context.Context, topDir, innerPath string, rpmIgnores map[string]types.IgnoreLists, errIgnores ...types.ErrIgnoreList) *types.ScanResult {
	baton := &Baton{TopDir: topDir}
	res := types.NewScanResult().SetPath(innerPath)

	path := filepath.Join(topDir, innerPath)

	// We are only interested in Linux binaries.
	elf, err := isElfExe(path, baton)
	if err != nil {
		return res.SetError(err)
	}
	if !elf {
		return res.Skipped()
	}

	goBinary, err := isGoExecutable(path, baton)
	if err != nil {
		return res.SetError(err)
	}
	var checks []ValidationFn
	if goBinary {
		checks = validationFns["go"]
	} else {
		checks = validationFns["exe"]
	}

checks:
	for _, fn := range checks {
		if err := fn(ctx, path, baton); err != nil {
			// See if the error is to be ignored.
			for _, list := range errIgnores {
				if list.Ignore(innerPath, err.Error) {
					continue checks
				}
			}
			if res.RPM == "" {
				// Find out which rpm the file belongs to. For performance reasons,
				// only do it for files that failed validation.
				rpm, rpmErr := rpm.NameFromFile(ctx, topDir, innerPath)
				if rpmErr != nil {
					klog.Info(rpmErr) // XXX: a minor warning.
				} else {
					res.SetRPM(rpm)
				}
			}
			// See if the error is to be ignored for the rpm.
			if res.RPM != "" && len(rpmIgnores) > 0 {
				if i, ok := rpmIgnores[res.RPM]; ok {
					if i.ErrIgnores.Ignore(innerPath, err.Error) {
						continue
					}
				}
			}
			return res.SetValidationError(err)
		}
	}

	return res.Success()
}

// newSemverConstraint is like semver.NewConstraint but panics if the expression cannot be parsed.
// It simplifies safe initialization of global variables holding preparsed constraints.
func newSemverConstraint(str string) *semver.Constraints {
	c, err := semver.NewConstraint(str)
	if err != nil {
		panic(fmt.Errorf("semver: can't parse constraint %v: %w", str, err))
	}
	return c
}
