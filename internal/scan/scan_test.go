package scan

import (
	"context"
	"testing"
	"time"

	v1 "github.com/openshift/api/image/v1"
	"github.com/openshift/check-payload/internal/types"
)

var (
	baseConfig = &types.Config{
		OutputFormat: "table",          // or another suitable format
		Parallelism:  1,                // for test simplicity, you might want to run sequentially
		TimeLimit:    30 * time.Second, // or a suitable duration
		Verbose:      true,             // if you want verbose output during testing
		ConfigFile: types.ConfigFile{
			PayloadIgnores:         make(map[string]types.IgnoreLists),
			TagIgnores:             make(map[string]types.IgnoreLists),
			RPMIgnores:             make(map[string]types.IgnoreLists),
			CertifiedDistributions: []string{"Red Hat Enterprise Linux release 9.2 (Plow)", "Red Hat Enterprise Linux CoreOS release 4.12"},
		},
	}
	ignoredOsConfig = &types.Config{
		OutputFormat: "table",
		Parallelism:  1,
		TimeLimit:    30 * time.Second,
		Verbose:      true,
		Components:   []string{"UnsupportedOperatingSystemIgnored"},
		ConfigFile: types.ConfigFile{
			PayloadIgnores: map[string]types.IgnoreLists{
				"UnsupportedOperatingSystemIgnored": {
					FilterFiles: make([]string, 0),
					FilterDirs:  make([]string, 0),
					ErrIgnores: types.ErrIgnoreList{{
						Error: types.KnownError{Err: types.ErrOSNotCertified},
						Files: make([]string, 0),
						Dirs:  make([]string, 0),
						// see scan.go line 193: the mock creates a Tag.Name = ""
						Tags: []string{""},
					}},
				},
			},
			RPMIgnores:             make(map[string]types.IgnoreLists),
			CertifiedDistributions: []string{"Red Hat Enterprise Linux release 12388.3 (Plow)"},
		},
	}
)

// TestRunLocalScan tests the RunLocalScan function with mock unpacked directories.
func TestRunLocalScan(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name                string
		mockUnpackedDirPath string
		mockConfig          *types.Config
		expectedResult      bool // true if scan should pass, false if it should fail
	}{
		{"GoodMockUnpackedDir", "../../test/resources/mock_unpacked_dir-1", baseConfig, true},
		{"BadMockUnpackedDir", "../../test/resources/mock_unpacked_dir-2", baseConfig, false},
		{"BadMockUnsupportedOperatingSystem", "../../test/resources/mock_unsupported_os", baseConfig, false},
		{"UnsupportedOperatingSystemIgnored", "../../test/resources/mock_unsupported_os", ignoredOsConfig, true},
		{"SymlinkedOsRelease", "../../test/resources/mock_os_symlinked", baseConfig, true},
	}
	// Iterate over test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup context and config
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Run the local scan.
			results := RunLocalScan(ctx, tc.mockConfig, tc.mockUnpackedDirPath)

			// Check if results meet expected criteria
			passed := !IsFailed(results)
			if passed != tc.expectedResult {
				t.Errorf("Test %s: expected pass = %t, got pass = %t", tc.name, tc.expectedResult, passed)
			}
		})
	}
}

func TestShouldSkipOSValidation(t *testing.T) {
	testCases := []struct {
		name      string
		config    *types.Config
		tag       *v1.TagReference
		component *types.OpenshiftComponent
		expected  bool
	}{
		{
			name:   "no tag should not skip",
			config: baseConfig,
			tag:    nil,
			component: &types.OpenshiftComponent{
				Component: "test-component",
			},
			expected: false,
		},
		{
			name:   "tag with no ignores should not skip",
			config: baseConfig,
			tag: &v1.TagReference{
				Name: "regular-tag",
			},
			component: &types.OpenshiftComponent{
				Component: "test-component",
			},
			expected: false,
		},
		{
			name: "rhel-coreos tag with tag-based ignore should skip",
			config: &types.Config{
				ConfigFile: types.ConfigFile{
					TagIgnores: map[string]types.IgnoreLists{
						"rhel-coreos": {
							ErrIgnores: types.ErrIgnoreList{{
								Error: types.KnownError{Err: types.ErrOSNotCertified},
								Tags:  []string{"rhel-coreos"},
							}},
						},
					},
					PayloadIgnores: make(map[string]types.IgnoreLists),
				},
			},
			tag: &v1.TagReference{
				Name: "rhel-coreos",
			},
			component: nil, // rhel-coreos has no component metadata
			expected:  true,
		},
		{
			name: "component with payload ignore should skip",
			config: &types.Config{
				ConfigFile: types.ConfigFile{
					PayloadIgnores: map[string]types.IgnoreLists{
						"test-component": {
							ErrIgnores: types.ErrIgnoreList{{
								Error: types.KnownError{Err: types.ErrOSNotCertified},
								Tags:  []string{"test-tag"},
							}},
						},
					},
					TagIgnores: make(map[string]types.IgnoreLists),
				},
			},
			tag: &v1.TagReference{
				Name: "test-tag",
			},
			component: &types.OpenshiftComponent{
				Component: "test-component",
			},
			expected: true,
		},
		{
			name: "component ignore with wrong tag should not skip",
			config: &types.Config{
				ConfigFile: types.ConfigFile{
					PayloadIgnores: map[string]types.IgnoreLists{
						"test-component": {
							ErrIgnores: types.ErrIgnoreList{{
								Error: types.KnownError{Err: types.ErrOSNotCertified},
								Tags:  []string{"different-tag"},
							}},
						},
					},
					TagIgnores: make(map[string]types.IgnoreLists),
				},
			},
			tag: &v1.TagReference{
				Name: "test-tag",
			},
			component: &types.OpenshiftComponent{
				Component: "test-component",
			},
			expected: false,
		},
		{
			name: "component ignore with wrong error should not skip",
			config: &types.Config{
				ConfigFile: types.ConfigFile{
					PayloadIgnores: map[string]types.IgnoreLists{
						"test-component": {
							ErrIgnores: types.ErrIgnoreList{{
								Error: types.KnownError{Err: types.ErrGoMissingTag}, // different error
								Tags:  []string{"test-tag"},
							}},
						},
					},
					TagIgnores: make(map[string]types.IgnoreLists),
				},
			},
			tag: &v1.TagReference{
				Name: "test-tag",
			},
			component: &types.OpenshiftComponent{
				Component: "test-component",
			},
			expected: false,
		},
		{
			name: "both component and tag ignores - component takes precedence",
			config: &types.Config{
				ConfigFile: types.ConfigFile{
					PayloadIgnores: map[string]types.IgnoreLists{
						"test-component": {
							ErrIgnores: types.ErrIgnoreList{{
								Error: types.KnownError{Err: types.ErrOSNotCertified},
								Tags:  []string{"test-tag"},
							}},
						},
					},
					TagIgnores: map[string]types.IgnoreLists{
						"test-tag": {
							ErrIgnores: types.ErrIgnoreList{{
								Error: types.KnownError{Err: types.ErrOSNotCertified},
								Tags:  []string{"test-tag"},
							}},
						},
					},
				},
			},
			tag: &v1.TagReference{
				Name: "test-tag",
			},
			component: &types.OpenshiftComponent{
				Component: "test-component",
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.config.ShouldIgnoreOSValidation(tc.tag, tc.component, types.ErrOSNotCertified)
			if result != tc.expected {
				t.Errorf("shouldSkipOSValidation() = %v, expected %v", result, tc.expected)
			}
		})
	}
}
