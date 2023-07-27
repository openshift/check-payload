package releases_test

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/openshift/check-payload/dist/releases"
	"github.com/openshift/check-payload/internal/types"
)

const mainConfig = "../../config.toml"

func decodeConfig(t *testing.T, file string) *types.ConfigFile {
	config := &types.ConfigFile{}
	res, err := toml.DecodeFile(file, &config)
	if err != nil {
		t.Errorf("invalid config file %q: %v", file, err)
	}
	if un := res.Undecoded(); len(un) != 0 {
		t.Errorf("unknown keys in config %q: %+v", file, un)
	}
	return config
}

// TestConfigs checks that all embedded configs are parsable, have no unknown
// keys, and that versioned configs contain no entries that the main config
// already has.
func TestConfigs(t *testing.T) {
	for i, dir := range releases.GetVersions() {
		main := decodeConfig(t, mainConfig)
		if i == 0 { // Validate main config only once.
			err, warn := main.Validate()
			if err != nil || warn != nil {
				t.Errorf("main config validation failed: %v; %v", err, warn)
			}
		}
		add := decodeConfig(t, dir+"/config.toml")
		if err, warn := add.Validate(); err != nil || warn != nil {
			t.Errorf("%s config failed validation: %v; %v", dir, err, warn)
		}
		err := main.Add(add)
		if err != nil {
			t.Errorf("%s config has duplicates: %v", dir, err)
		}
	}
}
