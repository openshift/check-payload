package scan

import (
	"context"
	"testing"
	"time"

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

	cfg := &types.Config{
		OutputFormat: "table",          // or another suitable format
		Parallelism:  1,                // for test simplicity, you might want to run sequentially
		TimeLimit:    30 * time.Second, // or a suitable duration
		Verbose:      true,             // if you want verbose output during testing
		ConfigFile: types.ConfigFile{
			PayloadIgnores: make(map[string]types.IgnoreLists),
			TagIgnores:     make(map[string]types.IgnoreLists),
			RPMIgnores:     make(map[string]types.IgnoreLists),
		},
	}
	// Iterate over test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup context and config
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Run the local scan.
			results := RunLocalScan(ctx, cfg, tc.mockUnpackedDirPath)

			// Check if results meet expected criteria
			passed := !IsFailed(results)
			if passed != tc.expectedResult {
				t.Errorf("Test %s: expected pass = %t, got pass = %t", tc.name, tc.expectedResult, passed)
			}
		})
	}
}
