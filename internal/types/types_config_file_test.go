package types_test

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift/check-payload/internal/types"
)

func decode(t *testing.T, src string) *types.ConfigFile {
	t.Helper()
	dst := &types.ConfigFile{}
	res, err := toml.Decode(src, &dst)
	require.NoError(t, err)
	require.Empty(t, res.Undecoded())
	err, warn := dst.Validate()
	require.NoError(t, err)
	require.NoError(t, warn)

	return dst
}

const (
	ex1 = `filter_files = [ "/some", "/files" ]
filter_dirs = [ "/some", "/dirs" ]
filter_images = [ "some", "images" ]

[payload.one]
  filter_files  = [ "/one_file" ]
  filter_dirs = [ "/one_dir" ]

[tag.smth]
  filter_files = [ "/smth_file1", "/smth_file2" ]
`
	ex2 = `filter_files = [ "/more" ]
filter_dirs = [ "/more" ]
filter_images = [ "more" ]

[payload.two]
  filter_files = [ "/two" ]
  filter_dirs = [ "/two" ]

[tag.smth]
  filter_dirs = [ "/smth_dir1" ]
`
	// This is ex1 + ex2
	ex1ex2 = `filter_files = ["/some", "/files", "/more"]
filter_dirs = [ "/some", "/dirs", "/more" ]
filter_images = [ "some", "images", "more" ]

[payload.one]
  filter_files  = [ "/one_file" ]
  filter_dirs = [ "/one_dir" ]

[payload.two]
  filter_files = [ "/two" ]
  filter_dirs = [ "/two" ]

[tag.smth]
  filter_files = ["/smth_file1", "/smth_file2"]
  filter_dirs = ["/smth_dir1"]
`

	// This is an example with ErrIgnores.
	ign1 = `
[[ignore]]
  error = "ErrLibcryptoSoMissing"
  files = [ "/1", "/2", "/3" ]

[[ignore]]
  error = "ErrLibcryptoMany"
  files = [ "/1", "/2", "/3" ]

[[payload.one.ignore]]
  error = "ErrNotDynLinked"
  files = [ "/one" ]

[[payload.one.ignore]]
  error = "ErrGoMissingTag"
  files = [ "/two/1" ]

[[tag.one.ignore]]
  error = "ErrLibcryptoMissing"
  files = [ "/foo/1", "/foo/2" ]

[[rpm.one.ignore]]
  error = "ErrGoNotCgoEnabled"
  files = [ "/one/11", "/one/22" ]
`

	// An addition to ign1.
	ign2 = `
[[ignore]]
  error = "ErrLibcryptoSoMissing"
  files = [ "/3", "/4", "/5", "/6" ] # /3 is a duplicate.
  dirs = [ "/dir1" ]

[[tag.two.ignore]]
  error = "ErrLibcryptoMissing"
  files = [ "/foo/3" ]
`

	// A merge of ign1 and ign2.
	ign1ign2 = `
[[ignore]]
  error = "ErrLibcryptoSoMissing"
  files = [ "/1", "/2", "/3", "/4", "/5", "/6" ]
  dirs = [ "/dir1" ]

[[ignore]]
  error = "ErrLibcryptoMany"
  files = [ "/1", "/2", "/3" ]

[[payload.one.ignore]]
  error = "ErrNotDynLinked"
  files = [ "/one" ]

[[payload.one.ignore]]
  error = "ErrGoMissingTag"
  files = [ "/two/1" ]

[[tag.one.ignore]]
  error = "ErrLibcryptoMissing"
  files = [ "/foo/1", "/foo/2" ]

[[tag.two.ignore]]
  error = "ErrLibcryptoMissing"
  files = [ "/foo/3" ]

[[rpm.one.ignore]]
  error = "ErrGoNotCgoEnabled"
  files = [ "/one/11", "/one/22" ]
`
)

func TestConfigMerge(t *testing.T) {
	testCases := []struct {
		name      string
		main, add string
		expected  string
		expWarns  bool
	}{
		{
			name: "empty configs",
		},
		{
			name:     "ex1 + empty add",
			main:     ex1,
			expected: ex1,
		},
		{
			name:     "empty main + ex1",
			add:      ex1,
			expected: ex1,
		},
		{
			name:     "ex1 + ex1",
			main:     ex1,
			add:      ex1,
			expected: ex1,
			expWarns: true,
		},
		{
			name:     "ex1 + ex2",
			main:     ex1,
			add:      ex2,
			expected: ex1ex2,
		},
		{
			name:     "ign1 + empty add",
			main:     ign1,
			expected: ign1,
		},
		{
			name:     "empty main + ign1",
			add:      ign1,
			expected: ign1,
		},
		{
			name:     "ign1 + ign1",
			main:     ign1,
			add:      ign1,
			expected: ign1,
			expWarns: true,
		},
		{
			name:     "ign1 + ign2",
			main:     ign1,
			add:      ign2,
			expected: ign1ign2,
			expWarns: true, // There are intentional duplicates.
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			mainCfg := decode(t, tc.main)
			addCfg := decode(t, tc.add)

			warns := mainCfg.Add(addCfg) // This is what we test.

			expCfg := decode(t, tc.expected)
			assert.Equal(t, expCfg, mainCfg)

			assert.Equal(t, tc.expWarns, warns != nil)
		})
	}
}
