package validations

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/openshift/check-payload/internal/golang"
	"github.com/openshift/check-payload/internal/types"
)

// MatchGoCryptoTargets matches exact package names and optional exact symbols.
// Only a leading vendor/ prefix is normalized; the reported symbol is unchanged.
func MatchGoCryptoTargets(functions []golang.Function, targets []types.GoCryptoTarget) []string {
	var matches []string
	for _, fn := range functions {
		for _, target := range targets {
			if strings.TrimPrefix(fn.Package, "vendor/") != target.Package {
				continue
			}
			if target.Symbol != "" && strings.TrimPrefix(fn.Symbol, "vendor/") != target.Symbol {
				continue
			}
			message := fmt.Sprintf("retained Go function %s: %s", fn.Symbol, target.Reason)
			if target.Reference != "" {
				message += " (" + target.Reference + ")"
			}
			matches = append(matches, message)
			break
		}
	}
	return matches
}

// ScanGoCryptoTargets returns ordinary scanner warnings, independent of FIPS
// build validation. No match is a detection result, not a compliance verdict.
func ScanGoCryptoTargets(root, innerPath string, targets []types.GoCryptoTarget) *types.ScanResult {
	res := types.NewScanResult().SetPath(innerPath)
	functions, err := golang.ReadFunctions(filepath.Join(root, innerPath))
	if err != nil {
		return res.SetValidationError(types.NewValidationError(fmt.Errorf("go crypto target detection incomplete: %w", err)).SetWarning())
	}
	if functions == nil {
		return nil
	}
	matches := MatchGoCryptoTargets(functions, targets)
	if len(matches) == 0 {
		return res.Success()
	}
	return res.SetValidationError(types.NewValidationError(fmt.Errorf("go crypto target evidence (requires review): %s", strings.Join(matches, "; "))).SetWarning())
}
