package scan

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/openshift/api/image/v1"
	"github.com/openshift/check-payload/internal/golang"
	"github.com/openshift/check-payload/internal/types"
)

func TestGoCryptoPhase(t *testing.T) {
	data, err := os.ReadFile("../../test/resources/fips_compliant_app")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	path := filepath.Join(root, "binary")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	functions, err := golang.ReadFunctions(path)
	if err != nil {
		t.Fatal(err)
	}
	var target types.GoCryptoTarget
	for _, fn := range functions {
		if fn.Package != "" {
			target = types.GoCryptoTarget{Package: fn.Package, Symbol: fn.Symbol, Reason: "test target"}
			break
		}
	}
	if target.Package == "" {
		t.Fatal("fixture has no package identities")
	}
	cfg := &types.Config{ConfigFile: types.ConfigFile{GoCryptoTargets: []types.GoCryptoTarget{target}}}
	tag := &v1.TagReference{Name: "pilot"}
	component := &types.OpenshiftComponent{Component: "pilot"}
	run := func(ctx context.Context) *types.ScanResults {
		results := types.NewScanResults()
		scanGoCryptoPhase(ctx, cfg, tag, component, root, results)
		return results
	}
	if got := run(context.Background()); len(got.Items) != 0 {
		t.Fatal("detector must be opt-in")
	}
	cfg.DetectGoCrypto = true
	got := run(context.Background())
	if len(got.Items) != 1 || !got.Items[0].IsLevel(types.Warning) || got.Items[0].Tag != tag || got.Items[0].Component != component {
		t.Fatalf("missing warning/provenance for non-executable file: %+v", got.Items)
	}
	if IsFailed([]*types.ScanResults{got}) || !IsWarnings([]*types.ScanResults{got}) {
		t.Fatal("matches must be warnings, not hard failures")
	}
	cfg.GoCryptoTargets[0].Package = "unrelated/package"
	cfg.GoCryptoTargets[0].Symbol = ""
	got = run(context.Background())
	if len(got.Items) != 1 || !got.Items[0].IsSuccess() {
		t.Fatal("no match should retain the existing success result")
	}
	cfg.FilterFiles = []string{"/binary"}
	if got := run(context.Background()); len(got.Items) != 0 {
		t.Fatal("file filters must apply")
	}
	cfg.FilterFiles = nil
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := run(ctx); !IsFailed([]*types.ScanResults{got}) {
		t.Fatal("canceled traversal must report incomplete coverage")
	}
}
