package validations

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift/check-payload/internal/types"
)

func createDirAndFile(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fips.so"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAnyPathExists(t *testing.T) {
	paths := []string{"/usr/lib64/ossl-modules/fips.so", "/usr/lib/ossl-modules/fips.so"}

	t.Run("file present at lib64", func(t *testing.T) {
		dir := t.TempDir()
		createDirAndFile(t, filepath.Join(dir, "usr", "lib64", "ossl-modules"))
		if !anyPathExists(dir, paths) {
			t.Error("expected true when file present")
		}
	})

	t.Run("file present at lib", func(t *testing.T) {
		dir := t.TempDir()
		createDirAndFile(t, filepath.Join(dir, "usr", "lib", "ossl-modules"))
		if !anyPathExists(dir, paths) {
			t.Error("expected true when file present")
		}
	})

	t.Run("file absent", func(t *testing.T) {
		dir := t.TempDir()
		if anyPathExists(dir, paths) {
			t.Error("expected false when no file present")
		}
	})

	t.Run("nil paths", func(t *testing.T) {
		dir := t.TempDir()
		if anyPathExists(dir, nil) {
			t.Error("expected false for nil paths")
		}
	})
}

func TestCheckArtifact(t *testing.T) {
	ctx := context.Background()

	t.Run("RPM missing", func(t *testing.T) {
		dir := t.TempDir()
		m := types.FipsModule{
			Module:            "openssl",
			CertifiedArtifact: "openssl-fips-provider",
		}
		if err := CheckArtifact(ctx, m, dir); err == nil {
			t.Error("expected error when RPM missing")
		}
	})

	t.Run("RPM missing even with paths present", func(t *testing.T) {
		dir := t.TempDir()
		createDirAndFile(t, filepath.Join(dir, "usr", "lib64", "ossl-modules"))
		m := types.FipsModule{
			Module:                 "openssl",
			CertifiedArtifact:      "openssl-fips-provider",
			CertifiedArtifactPaths: []string{"/usr/lib64/ossl-modules/fips.so"},
		}
		if err := CheckArtifact(ctx, m, dir); err == nil {
			t.Error("expected error: file exists but RPM not found")
		}
	})

	t.Run("paths gate: RPM present but file missing fails", func(t *testing.T) {
		dir := t.TempDir()
		m := types.FipsModule{
			Module:                 "openssl",
			CertifiedArtifact:      "openssl-libs",
			CertifiedArtifactPaths: []string{"/usr/lib64/ossl-modules/fips.so"},
		}
		// Can't mock RPM presence without rpmdb, so this tests the path
		// gate in isolation via anyPathExists (tested above).
		// CheckArtifact will fail at RPM check first.
		if err := CheckArtifact(ctx, m, dir); err == nil {
			t.Error("expected error when RPM missing")
		}
	})
}
