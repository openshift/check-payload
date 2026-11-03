package golang

import (
	"compress/gzip"
	"debug/elf"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadFunctions(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":           "module example.com/pilot\n\ngo 1.26.0\n",
		"main.go":          "package main\nimport (\"fmt\"; \"example.com/pilot/primitive\")\nfunc main() { fmt.Println(primitive.Key()) }\n",
		"primitive/key.go": "package primitive\n//go:noinline\nfunc Key() int { return 42 }\n//go:noinline\nfunc Unused() int { return 99 }\n",
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var want []Function
	for _, target := range []struct{ os, arch, flags string }{
		{"linux", "amd64", ""},
		{"linux", "amd64", "-s -w"},
		{"darwin", "arm64", "-s -w"},
	} {
		t.Run(target.os+target.flags, func(t *testing.T) {
			path := filepath.Join(dir, target.os+target.arch+target.flags)
			cmd := exec.Command("go", "build", "-ldflags="+target.flags, "-o", path, ".")
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "GOOS="+target.os, "GOARCH="+target.arch, "CGO_ENABLED=0", "GOFLAGS=", "GOWORK=off")
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("build fixture: %v\n%s", err, output)
			}
			if err := os.Chmod(path, 0o600); err != nil {
				t.Fatal(err)
			}
			functions, err := ReadFunctions(path)
			if err != nil {
				t.Fatal(err)
			}
			var got []Function
			for _, fn := range functions {
				if fn.Package == "example.com/pilot/primitive" {
					got = append(got, fn)
				}
			}
			if len(got) != 1 || got[0].Symbol != "example.com/pilot/primitive.Key" {
				t.Fatalf("want retained Key and eliminated Unused, got %+v", got)
			}
			if want == nil {
				want = got
			} else if !reflect.DeepEqual(want, got) {
				t.Fatalf("stripping/platform changed identities: %+v != %+v", got, want)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			packed, err := os.Create(path + ".gz")
			if err != nil {
				t.Fatal(err)
			}
			zr := gzip.NewWriter(packed)
			if _, err := zr.Write(data); err != nil {
				t.Fatal(err)
			}
			if err := zr.Close(); err != nil {
				t.Fatal(err)
			}
			if err := packed.Close(); err != nil {
				t.Fatal(err)
			}
			compressed, err := ReadFunctions(path + ".gz")
			if err != nil || !reflect.DeepEqual(functions, compressed) {
				t.Fatalf("gzip changed identities: %v", err)
			}
			if target.os == "linux" {
				exe, err := elf.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				section := exe.Section(".gopclntab")
				if section == nil {
					t.Fatal("fixture missing pclntab")
				}
				for i := section.Offset; i < section.Offset+section.Size; i++ {
					data[i] = 0
				}
				exe.Close()
				if err := os.WriteFile(path, data, 0o600); err != nil {
					t.Fatal(err)
				}
				if _, err := ReadFunctions(path); err == nil {
					t.Fatal("damaged Go metadata must not produce a clean no-match")
				}
			}
		})
	}
}

func TestReadFunctionsNonGo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data")
	if err := os.WriteFile(path, []byte(strings.Repeat("data", 20)), 0o600); err != nil {
		t.Fatal(err)
	}
	if functions, err := ReadFunctions(path); err != nil || functions != nil {
		t.Fatalf("non-Go file: %+v, %v", functions, err)
	}
	if _, err := ReadFunctions(path + "-missing"); err == nil {
		t.Fatal("missing input must report an error")
	}
}
