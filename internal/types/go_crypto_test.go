package types_test

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/openshift/check-payload/internal/types"
)

func TestGoCryptoConfig(t *testing.T) {
	cfg := decode(t, `[[go_crypto_targets]]
package = "example.org/primitive"
symbol = "example.org/primitive.Key"
reason = "Review security use"
reference = "ticket"
`)
	if len(cfg.GoCryptoTargets) != 1 || cfg.GoCryptoTargets[0].Symbol != "example.org/primitive.Key" {
		t.Fatal("target configuration lost")
	}
	other := &types.ConfigFile{GoCryptoTargets: []types.GoCryptoTarget{{Package: "another/package", Reason: "review"}}}
	if err := cfg.Add(other); err != nil || len(cfg.GoCryptoTargets) != 2 {
		t.Fatalf("version config merge: %v", err)
	}
	for _, target := range []types.GoCryptoTarget{
		{Reason: "review"},
		{Package: "example.org/primitive"},
		{Package: "example.org/*", Reason: "review"},
		{Package: "vendor/example.org/primitive", Reason: "review"},
		{Package: "example.org/primitive", Symbol: "other/package.Key", Reason: "review"},
	} {
		cfg.GoCryptoTargets = []types.GoCryptoTarget{target}
		if err, _ := cfg.Validate(); err == nil {
			t.Fatalf("invalid selector accepted: %+v", target)
		}
	}
	var empty types.ConfigFile
	if _, err := toml.Decode("", &empty); err != nil || len(empty.GoCryptoTargets) != 0 {
		t.Fatal("empty config must not enable targets")
	}
}
