package inventory

import (
	"bytes"
	"compress/gzip"
	"context"
	"debug/elf"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func runGo(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=", "GOFIPS140=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go %v: %v\n%s", args, err, out)
	}
}

func TestSourceUsesExecutableReachability(t *testing.T) {
	root, err := filepath.Abs("testdata/source")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		root string
		want bool
	}{{"./cmd/unreachable", false}, {"./cmd/reachable", true}} {
		t.Run(tc.root, func(t *testing.T) {
			artifacts, err := Source(context.Background(), SourceOptions{Dir: root, Patterns: []string{tc.root}, Env: []string{"GOWORK=off", "GOFLAGS=", "GOFIPS140=off"}})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, f := range artifacts[0].Findings {
				if f.Symbol == xcrypto+"argon2.Key" {
					found = true
					if f.Classification != "F1" || len(f.CallChain) < 2 {
						t.Fatalf("missing classification or root-to-call evidence: %+v", f)
					}
				}
			}
			if found != tc.want {
				t.Fatalf("primitive finding=%v, want %v; imports alone must not be findings", found, tc.want)
			}
		})
	}
	if _, err := Source(context.Background(), SourceOptions{Dir: root, Patterns: []string{"./missing"}, Env: []string{"GOWORK=off"}}); err == nil {
		t.Fatal("missing executable root reported as clean")
	}
}

func TestBinaryStrippingCompressionAndLocalCoverage(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ELF fixture build")
	}
	dir := t.TempDir()
	root, err := filepath.Abs("testdata/source")
	if err != nil {
		t.Fatal(err)
	}
	plain, stripped := filepath.Join(dir, "plain"), filepath.Join(dir, "stripped")
	runGo(t, root, "build", "-o", plain, "./cmd/reachable")
	runGo(t, root, "build", "-ldflags=-s -w", "-o", stripped, "./cmd/reachable")
	left, right := Binary(plain), Binary(stripped)
	if left.Coverage != "analyzed" || right.Coverage != "analyzed" {
		t.Fatalf("stripping broke coverage: %+v / %+v", left.Errors, right.Errors)
	}
	symbols := func(a Artifact) []string {
		var names []string
		for _, f := range a.Findings {
			names = append(names, f.Symbol)
		}
		return names
	}
	if !reflect.DeepEqual(symbols(left), symbols(right)) || len(right.Findings) == 0 {
		t.Fatal("stripped runtime metadata lost crypto functions")
	}
	data, err := os.ReadFile(stripped)
	if err != nil {
		t.Fatal(err)
	}
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	if _, err := zw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(dir, "transported.gz")
	if err := os.WriteFile(zipPath, compressed.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	zipped := Binary(zipPath)
	if zipped.Coverage != "analyzed" || !reflect.DeepEqual(symbols(right), symbols(zipped)) {
		t.Fatalf("gzip transport changed inventory: %+v", zipped)
	}
	artifacts := Local(context.Background(), dir, "sha256:fixture")
	if len(artifacts) != 3 {
		t.Fatalf("transported binary without execute bits was missed: %d artifacts", len(artifacts))
	}
	for _, a := range artifacts {
		if !strings.HasPrefix(a.Path, "/") || a.Image != "sha256:fixture" {
			t.Fatalf("missing image-relative provenance: %+v", a)
		}
	}
	checked := RequirePaths(artifacts, []string{"/plain", "/missing", "/missing"}, "sha256:fixture")
	if len(checked) != 4 || checked[3].Coverage != "failed" || checked[3].Path != "/missing" {
		t.Fatal("a discovered executable concealed a missing expected binary")
	}
	if !NewReport("local", "fixture").Incomplete() {
		t.Fatal("empty coverage treated as complete")
	}
	missing := Binary(filepath.Join(dir, "missing"))
	if missing.Coverage != "failed" || len(missing.Errors) == 0 {
		t.Fatal("missing binary treated as clean")
	}
}

func TestContextSensitiveClassification(t *testing.T) {
	withoutCaller := classify(xcrypto+"blowfish", xcrypto+"blowfish.NewCipher", "v0.52.0", nil)
	if withoutCaller.Classification != "unresolved" {
		t.Fatal("binary package co-presence must not grant a substrate exception")
	}
	withCaller := classify(xcrypto+"blowfish", xcrypto+"blowfish.NewCipher", "v0.52.0", []string{xcrypto + "bcrypt.GenerateFromPassword"})
	if withCaller.Classification != "unresolved" || withCaller.Guard != "unresolved" || !reflect.DeepEqual(withCaller.Candidates, []string{"F1", "F2"}) {
		t.Fatal("bcrypt substrate should be an F2 candidate without granting an exception")
	}
	old := classify(xcrypto+"hkdf", xcrypto+"hkdf.New", "v0.10.0", nil)
	fork := classify(xcrypto+"hkdf", xcrypto+"hkdf.New", "replacement:example/fork@v0.52.0", nil)
	modern := classify(xcrypto+"hkdf", xcrypto+"hkdf.New", "v0.52.0", nil)
	if old.Classification != "unresolved" || fork.Classification != "unresolved" || modern.Classification != "F3" {
		t.Fatal("delegation must not be inferred from package name alone")
	}
	keccak := classify(xcrypto+"sha3", xcrypto+"sha3.NewLegacyKeccak256", "v0.52.0", nil)
	if keccak.Classification != "F1" {
		t.Fatal("legacy Keccak must not inherit wrapper classification")
	}
	unknown := classify(xcrypto+"future", xcrypto+"future.Exchange", "v0.52.0", nil)
	if unknown.Classification != "unresolved" {
		t.Fatal("unknown crypto package classified as acceptable")
	}
}

func TestTransportedMachOAndDamagedELF(t *testing.T) {
	root, err := filepath.Abs("testdata/source")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	mac := filepath.Join(dir, "darwin-server")
	cmd := exec.Command("go", "build", "-ldflags=-s -w", "-o", mac, "./cmd/reachable")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=", "GOFIPS140=off", "GOOS=darwin", "GOARCH=arm64", "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cross build: %v: %s", err, out)
	}
	a := Binary(mac)
	if a.Coverage != "analyzed" || len(a.Findings) == 0 {
		t.Fatalf("transported Mach-O coverage: %+v", a)
	}
	if runtime.GOOS != "linux" {
		return
	}
	binary := filepath.Join(dir, "damaged")
	runGo(t, root, "build", "-o", binary, "./cmd/reachable")
	file, err := elf.Open(binary)
	if err != nil {
		t.Fatal(err)
	}
	section := file.Section(".gopclntab")
	if section == nil {
		t.Fatal("fixture has no pclntab")
	}
	offset, size := section.Offset, section.Size
	file.Close()
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	clear(data[offset : offset+size])
	if err := os.WriteFile(binary, data, 0o600); err != nil {
		t.Fatal(err)
	}
	a = Binary(binary)
	if a.Coverage != "failed" || len(a.Errors) == 0 {
		t.Fatal("damaged function metadata reported as clean")
	}
}

func TestStandardLibraryCryptoOrigin(t *testing.T) {
	f := classify(xcrypto+"chacha20poly1305", "vendor/"+xcrypto+"chacha20poly1305.New", "stdlib-vendor:go1.26.0", nil)
	if f.Origin != "stdlib-vendor" || f.ModuleVersion != "stdlib-vendor:go1.26.0" {
		t.Fatalf("stdlib crypto mistaken for module dependency: %+v", f)
	}
}
