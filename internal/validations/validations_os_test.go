package validations

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/openshift/check-payload/internal/types"
)

func TestValidateModuleArtifacts(t *testing.T) {
	ctx := context.Background()

	baseCfg := func(modules []types.FipsModule) *types.Config {
		return &types.Config{
			ConfigFile: types.ConfigFile{
				FIPSCertifiedModules: modules,
			},
		}
	}

	t.Run("no modules in use returns nil", func(t *testing.T) {
		dir := t.TempDir()
		cfg := baseCfg([]types.FipsModule{
			{Module: "openssl", CertifiedArtifact: "openssl-fips-provider"},
		})
		if ve := ValidateModuleArtifacts(ctx, cfg, dir, nil); ve != nil {
			t.Errorf("expected nil, got %v", ve)
		}
	})

	t.Run("module matched + RPM missing", func(t *testing.T) {
		dir := t.TempDir()
		cfg := baseCfg([]types.FipsModule{
			{Module: "openssl", CertifiedArtifact: "openssl-fips-provider"},
		})
		ve := ValidateModuleArtifacts(ctx, cfg, dir, []string{"openssl"})
		if ve == nil {
			t.Error("expected error when RPM missing")
		}
	})

	t.Run("unmatched module returns nil", func(t *testing.T) {
		dir := t.TempDir()
		cfg := baseCfg([]types.FipsModule{
			{Module: "go", CertifiedArtifact: "go-std"},
		})
		if ve := ValidateModuleArtifacts(ctx, cfg, dir, []string{"openssl"}); ve != nil {
			t.Errorf("expected nil when no config module matches, got %v", ve)
		}
	})
}

func TestValidateModule(t *testing.T) {
	ctx := context.Background()

	baseCfg := func(modules []types.FipsModule) *types.Config {
		return &types.Config{
			ConfigFile: types.ConfigFile{
				FIPSCertifiedModules: modules,
			},
		}
	}

	t.Run("binary source skips image check", func(t *testing.T) {
		dir := t.TempDir()
		cfg := baseCfg([]types.FipsModule{
			{Module: "go", ArtifactSource: "binary", CertifiedArtifact: "crypto/fips140"},
		})
		if ve := ValidateModule(ctx, cfg, dir, "go"); ve != nil {
			t.Errorf("expected nil for binary source, got %v", ve)
		}
	})

	t.Run("no artifact and no host lib fails", func(t *testing.T) {
		dir := t.TempDir()
		cfg := baseCfg([]types.FipsModule{
			{
				Module:            "openssl",
				ArtifactSource:    "image",
				CertifiedArtifact: "openssl-fips-provider",
			},
		})
		if ve := ValidateModule(ctx, cfg, dir, "openssl"); ve == nil {
			t.Error("expected error when no artifact and no host lib")
		}
	})

	t.Run("anyPathExists gates CheckArtifact", func(t *testing.T) {
		dir := t.TempDir()
		paths := []string{"/usr/lib64/ossl-modules/fips.so"}

		if anyPathExists(dir, paths) {
			t.Error("expected false when fips.so absent")
		}
		createDirAndFile(t, filepath.Join(dir, "usr", "lib64", "ossl-modules"))
		if !anyPathExists(dir, paths) {
			t.Error("expected true when fips.so present")
		}
	})
}
