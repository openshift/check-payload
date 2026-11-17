package validations

import (
	"strings"
	"testing"

	"github.com/openshift/check-payload/internal/golang"
	"github.com/openshift/check-payload/internal/types"
)

func TestMatchGoCryptoTargets(t *testing.T) {
	functions := []golang.Function{
		{Package: "example.org/primitive", Symbol: "example.org/primitive.Key"},
		{Package: "example.org/primitive", Symbol: "example.org/primitive.Other"},
		{Package: "example.org/primitive/sub", Symbol: "example.org/primitive/sub.Key"},
		{Package: "vendor/example.org/primitive", Symbol: "vendor/example.org/primitive.Key"},
		{Package: "other/vendor/example.org/primitive", Symbol: "other/vendor/example.org/primitive.Key"},
	}
	target := types.GoCryptoTarget{Package: "example.org/primitive", Symbol: "example.org/primitive.Key", Reason: "review use", Reference: "ticket"}
	got := MatchGoCryptoTargets(functions, []types.GoCryptoTarget{target})
	if len(got) != 2 || !strings.Contains(got[1], "vendor/example.org/primitive.Key") || !strings.Contains(got[0], "review use (ticket)") {
		t.Fatalf("exact matching/vendor identity/explanation: %+v", got)
	}
	target.Symbol = ""
	if got := MatchGoCryptoTargets(functions, []types.GoCryptoTarget{target}); len(got) != 3 {
		t.Fatalf("package selector should match only exact package: %+v", got)
	}
	if got := MatchGoCryptoTargets(functions, nil); len(got) != 0 {
		t.Fatalf("no targets: %+v", got)
	}
}

func TestGoCryptoReadFailureIsWarning(t *testing.T) {
	res := ScanGoCryptoTargets(t.TempDir(), "/missing", []types.GoCryptoTarget{{Package: "example.org/primitive", Reason: "review"}})
	if res == nil || !res.IsLevel(types.Warning) || !strings.Contains(res.Error.Error.Error(), "detection incomplete") {
		t.Fatalf("read failure must be incomplete detection, got %+v", res)
	}
}
