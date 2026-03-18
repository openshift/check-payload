package validations

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift/check-payload/internal/types"
)

func setupFakeImage(t *testing.T, release string, providerPath string) string {
	t.Helper()
	dir := t.TempDir()
	etc := filepath.Join(dir, "etc")
	if err := os.MkdirAll(etc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(etc, "redhat-release"), []byte(release), 0o644); err != nil {
		t.Fatal(err)
	}
	if providerPath != "" {
		full := filepath.Join(dir, providerPath)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestValidateModuleArtifacts(t *testing.T) {
	ctx := context.Background()

	baseCfg := func(modules []types.FipsModule) *types.Config {
		return &types.Config{
			ConfigFile: types.ConfigFile{
				FIPSValidationMode:   "module",
				FIPSCertifiedModules: modules,
			},
		}
	}

	opensslModule := types.FipsModule{
		Module:            "openssl",
		CertifiedArtifact: "openssl-fips-provider",
		CertifiedArtifactPaths: []string{
			"/usr/lib64/ossl-modules/fips.so",
			"/usr/lib/ossl-modules/fips.so",
		},
	}

	opensslModuleWithVersion := types.FipsModule{
		Module:                      "openssl",
		CertifiedArtifact:           "openssl-fips-provider",
		CertifiedArtifactMinVersion: "3.0.7",
		CertifiedArtifactPaths: []string{
			"/usr/lib64/ossl-modules/fips.so",
		},
	}

	t.Run("no modules in use returns nil", func(t *testing.T) {
		dir := t.TempDir()
		cfg := baseCfg([]types.FipsModule{opensslModule})
		if ve := ValidateModuleArtifacts(ctx, cfg, dir, nil); ve != nil {
			t.Errorf("expected nil, got %v", ve)
		}
	})

	t.Run("module matched + provider present", func(t *testing.T) {
		dir := setupFakeImage(t, "", "usr/lib64/ossl-modules/fips.so")
		cfg := baseCfg([]types.FipsModule{opensslModule})
		if ve := ValidateModuleArtifacts(ctx, cfg, dir, []string{"openssl"}); ve != nil {
			t.Errorf("expected nil, got %v", ve)
		}
	})

	t.Run("module matched + provider missing", func(t *testing.T) {
		dir := t.TempDir()
		cfg := baseCfg([]types.FipsModule{opensslModule})
		ve := ValidateModuleArtifacts(ctx, cfg, dir, []string{"openssl"})
		if ve == nil {
			t.Error("expected error when provider missing")
		}
	})

	t.Run("version required but unknown from file path", func(t *testing.T) {
		dir := setupFakeImage(t, "", "usr/lib64/ossl-modules/fips.so")
		cfg := baseCfg([]types.FipsModule{opensslModuleWithVersion})
		ve := ValidateModuleArtifacts(ctx, cfg, dir, []string{"openssl"})
		if ve == nil {
			t.Error("expected error when version required but unknown")
		}
	})

	t.Run("unmatched module returns nil", func(t *testing.T) {
		dir := t.TempDir()
		goModule := types.FipsModule{Module: "go", CertifiedArtifact: "go-std"}
		cfg := baseCfg([]types.FipsModule{goModule})
		if ve := ValidateModuleArtifacts(ctx, cfg, dir, []string{"openssl"}); ve != nil {
			t.Errorf("expected nil when no config module matches, got %v", ve)
		}
	})
}
