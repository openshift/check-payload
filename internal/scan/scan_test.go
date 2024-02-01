package scan

import (
	"context"
	"testing"

	"github.com/openshift/check-payload/internal/types"
)

// TestRunLocalScan tests the RunLocalScan function with mock bundles.
// TODO add non-FIPS compliant compiled binary to BadBundle to make a fail test case
// TODO add FIPS compliant compiled binary to GoodBundle to make a better pass test case
func TestRunLocalScan(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name           string
		bundlePath     string
		expectedResult bool // true if scan should pass, false if it should fail
	}{
		{"GoodBundle", "../../test/resources/bundle-1", true},
		{"BadBundle", "../../test/resources/bundle-2", false},
	}

	// Iterate over test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup context and config
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var cfg types.Config
			cfg.NewDefaultConfig() // Assuming a method to set up default config

			// Run the local scan
			results := RunLocalScan(ctx, &cfg, tc.bundlePath)

			// Check if results meet expected criteria
			passed := !IsFailed(results)
			if passed != tc.expectedResult {
				t.Errorf("Test %s: expected pass = %t, got pass = %t", tc.name, tc.expectedResult, passed)
			}
		})
	}
}
