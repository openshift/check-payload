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

var testModule = types.FipsModule{
	Module:            "openssl",
	CertifiedArtifact: "openssl-fips-provider",
	CertifiedArtifactPaths: []string{
		"/usr/lib64/ossl-modules/fips.so",
		"/usr/lib/ossl-modules/fips.so",
	},
}

func TestArtifactPresent(t *testing.T) {
	ctx := context.Background()

	t.Run("no rpm db and no file path", func(t *testing.T) {
		dir := t.TempDir()
		_, present := artifactPresentAndVersion(ctx, dir, testModule)
		if present {
			t.Error("expected false when no db and no provider file")
		}
	})

	t.Run("file path under lib64", func(t *testing.T) {
		dir := t.TempDir()
		createDirAndFile(t, filepath.Join(dir, "usr", "lib64", "ossl-modules"))
		_, present := artifactPresentAndVersion(ctx, dir, testModule)
		if !present {
			t.Error("expected true when file present at configured path")
		}
	})

	t.Run("file path under lib", func(t *testing.T) {
		dir := t.TempDir()
		createDirAndFile(t, filepath.Join(dir, "usr", "lib", "ossl-modules"))
		_, present := artifactPresentAndVersion(ctx, dir, testModule)
		if !present {
			t.Error("expected true when file present at configured path")
		}
	})

	t.Run("no paths configured means file fallback skipped", func(t *testing.T) {
		dir := t.TempDir()
		createDirAndFile(t, filepath.Join(dir, "usr", "lib64", "ossl-modules"))
		noPaths := types.FipsModule{
			Module:            "openssl",
			CertifiedArtifact: "openssl-fips-provider",
		}
		_, present := artifactPresentAndVersion(ctx, dir, noPaths)
		if present {
			t.Error("expected false when no paths configured and no RPM db")
		}
	})
}

func TestCheckArtifact(t *testing.T) {
	ctx := context.Background()

	t.Run("artifact missing", func(t *testing.T) {
		dir := t.TempDir()
		err := CheckArtifact(ctx, testModule, dir)
		if err == nil {
			t.Error("expected error for missing artifact")
		}
	})

	t.Run("artifact present via file path", func(t *testing.T) {
		dir := t.TempDir()
		createDirAndFile(t, filepath.Join(dir, "usr", "lib64", "ossl-modules"))
		err := CheckArtifact(ctx, testModule, dir)
		if err != nil {
			t.Errorf("expected nil with artifact present, got %v", err)
		}
	})

	t.Run("version required but unknown from file path", func(t *testing.T) {
		dir := t.TempDir()
		createDirAndFile(t, filepath.Join(dir, "usr", "lib64", "ossl-modules"))
		m := testModule
		m.CertifiedArtifactMinVersion = "3.0.7"
		err := CheckArtifact(ctx, m, dir)
		if err == nil {
			t.Error("expected error when version required but unknown")
		}
	})
}
