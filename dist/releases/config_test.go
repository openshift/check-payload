package releases_test

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/openshift/check-payload/dist/releases"
	"github.com/openshift/check-payload/internal/types"
)

const mainConfig = "../../config.toml"

func testConfigIsSane(t *testing.T, file string) {
	config := &types.ConfigFile{}
	res, err := toml.DecodeFile(file, &config)
	if err != nil {
		t.Errorf("invalid config file %q: %v", file, err)
	}
	if un := res.Undecoded(); len(un) != 0 {
		t.Errorf("unknown keys in config %q: %+v", file, un)
	}
}

func TestConfigsAreSane(t *testing.T) {
	testConfigIsSane(t, mainConfig)
	for _, dir := range releases.GetVersions() {
		testConfigIsSane(t, dir+"/config.toml")
	}
}
