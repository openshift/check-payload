package inventory

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"debug/gosym"
	"debug/macho"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/check-payload/internal/golang"
)

const maxUncompressedBinary = 512 << 20

// Binary inventories a Go executable, including an executable stored as gzip.
func Binary(path string) Artifact {
	a := Artifact{Path: path, Coverage: "failed", Findings: []Finding{}}
	data, err := os.ReadFile(path)
	if err != nil {
		a.Errors = []string{err.Error()}
		return a
	}
	a.SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
	actualPath := path
	if bytes.HasPrefix(data, []byte{0x1f, 0x8b}) {
		actualPath, err = unpackGzip(data)
		if err != nil {
			a.Errors = []string{err.Error()}
			return a
		}
		defer os.Remove(actualPath)
	}
	bi, err := buildinfo.ReadFile(actualPath)
	if err != nil {
		unpacked := data
		if actualPath != path {
			unpacked, _ = os.ReadFile(actualPath)
		}
		if !bytes.Contains(unpacked, []byte("Go buildinf:")) && !bytes.Contains(unpacked, []byte("Go build ID:")) {
			a.Coverage = "not-go"
		}
		a.Errors = []string{fmt.Sprintf("cannot read Go build provenance: %v", err)}
		return a
	}
	a.GoVersion = bi.GoVersion
	a.BuildSettings = make(map[string]string)
	a.BuildSettings["mainPackage"] = bi.Path
	a.BuildSettings["mainModule"] = bi.Main.Path
	a.BuildSettings["mainModuleVersion"] = bi.Main.Version
	for _, setting := range bi.Settings {
		a.BuildSettings[setting.Key] = setting.Value
		if setting.Key == "vcs.revision" {
			a.SourceCommit = setting.Value
		}
	}
	table, err := readFunctions(actualPath, bi)
	if err != nil {
		a.Errors = []string{err.Error()}
		return a
	}
	version := ""
	for _, dep := range bi.Deps {
		if dep.Path == "golang.org/x/crypto" {
			version = dep.Version
			if dep.Replace != nil {
				// A replacement may be a fork even when it claims the same version.
				version = "replacement:" + dep.Replace.Path + "@" + dep.Replace.Version
			}
		}
	}
	for _, fn := range table.Funcs {
		pkg := packageFromSymbol(fn.Name)
		if pkg == "" {
			continue
		}
		finding := classify(pkg, fn.Name, version, nil)
		if finding.Origin == "stdlib-vendor" {
			finding = classify(pkg, fn.Name, "stdlib-vendor:"+bi.GoVersion, nil)
		}
		finding.Evidence = "linked-function"
		a.Findings = append(a.Findings, finding)
	}
	a.Coverage = "analyzed"
	return a
}

func unpackGzip(data []byte) (string, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("opening gzip executable: %w", err)
	}
	defer zr.Close()
	tmp, err := os.CreateTemp("", "crypto-inventory-*")
	if err != nil {
		return "", fmt.Errorf("creating decompression file: %w", err)
	}
	n, copyErr := io.Copy(tmp, io.LimitReader(zr, maxUncompressedBinary+1))
	closeErr := tmp.Close()
	if copyErr != nil || closeErr != nil || n > maxUncompressedBinary {
		os.Remove(tmp.Name())
		if err := errors.Join(copyErr, closeErr); err != nil {
			return "", fmt.Errorf("decompressing executable: %w", err)
		}
		return "", fmt.Errorf("decompressed executable exceeds %d bytes", maxUncompressedBinary)
	}
	return tmp.Name(), nil
}

func readFunctions(path string, bi *buildinfo.BuildInfo) (table *gosym.Table, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			table = nil
			err = fmt.Errorf("corrupt runtime function metadata in %s: %v", path, recovered)
		}
	}()
	if table, err := golang.ReadTable(path, bi); err == nil {
		if table == nil || len(table.Funcs) == 0 {
			return nil, fmt.Errorf("empty ELF runtime function metadata in %s", path)
		}
		return table, nil
	}
	// Installer images carry Darwin executables as data rather than running them.
	file, err := macho.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read runtime function metadata in %s; supported formats are Go ELF and Mach-O: %w", path, err)
	}
	defer file.Close()
	section, text := file.Section("__gopclntab"), file.Section("__text")
	if section == nil || text == nil {
		return nil, fmt.Errorf("Mach-O executable %s has no Go runtime function metadata", path)
	}
	data, err := section.Data()
	if err != nil {
		return nil, fmt.Errorf("reading Mach-O function metadata: %w", err)
	}
	table, err = gosym.NewTable(nil, gosym.NewLineTable(data, text.Addr))
	if err != nil {
		return nil, fmt.Errorf("invalid Mach-O runtime function metadata in %s: %w", path, err)
	}
	if table == nil || len(table.Funcs) == 0 {
		return nil, fmt.Errorf("empty Mach-O runtime function metadata in %s", path)
	}
	return table, nil
}

// Local inventories Go binaries in an unpacked rootfs, including transported
// executables without execute bits. Symlinks are not followed outside the rootfs.
func Local(ctx context.Context, root, image string) []Artifact {
	artifacts := []Artifact{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			artifacts = append(artifacts, Artifact{Path: path, Image: image, Coverage: "failed", Errors: []string{walkErr.Error()}, Findings: []Finding{}})
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		var magic [4]byte
		_, readErr := io.ReadFull(file, magic[:])
		file.Close()
		if readErr != nil {
			return nil
		}
		candidate := bytes.Equal(magic[:], []byte{0x7f, 'E', 'L', 'F'}) ||
			bytes.Equal(magic[:], []byte{0xcf, 0xfa, 0xed, 0xfe}) ||
			bytes.Equal(magic[:], []byte{0xce, 0xfa, 0xed, 0xfe}) ||
			bytes.Equal(magic[:2], []byte{'M', 'Z'}) ||
			(strings.HasSuffix(path, ".gz") && magic[0] == 0x1f && magic[1] == 0x8b)
		if !candidate {
			return nil
		}
		a := Binary(path)
		// Preserve errors for damaged Go build evidence, not ordinary C binaries.
		if a.Coverage == "not-go" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		a.Path = "/" + filepath.ToSlash(rel)
		a.Image = image
		artifacts = append(artifacts, a)
		return nil
	})
	if err != nil {
		artifacts = append(artifacts, Artifact{Path: root, Image: image, Coverage: "failed", Errors: []string{err.Error()}, Findings: []Finding{}})
	}
	if len(artifacts) == 0 {
		artifacts = append(artifacts, Artifact{Path: root, Image: image, Coverage: "unresolved", Errors: []string{"no identifiable Go executables found; verify expected binary coverage"}, Findings: []Finding{}})
	}
	return artifacts
}
