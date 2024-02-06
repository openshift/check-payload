package scan

import (
	"context"
	"testing"

	"github.com/openshift/check-payload/internal/types"
)

// TestRunLocalScan tests the RunLocalScan function with mock unpacked directories.
func TestRunLocalScan(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name                string
		mockUnpackedDirPath string
		expectedResult      bool // true if scan should pass, false if it should fail
	}{
		{"GoodMockUnpackedDir", "../../test/resources/mock_unpacked_dir-1", true},
		{"BadMockUnpackedDir", "../../test/resources/mock_unpacked_dir-2", false},
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
			results := RunLocalScan(ctx, &cfg, tc.mockUnpackedDirPath)

			// Check if results meet expected criteria
			passed := !IsFailed(results)
			if passed != tc.expectedResult {
				t.Errorf("Test %s: expected pass = %t, got pass = %t", tc.name, tc.expectedResult, passed)
			}
		})
	}
}
