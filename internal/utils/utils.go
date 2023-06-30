package utils

import (
	"errors"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"k8s.io/klog/v2"

	"github.com/openshift/check-payload/internal/scan"
)

func GetConfig(embeddedConfig, defaultConfigFile string, config *scan.Config) error {
	file := defaultConfigFile
	if file == "" {
		file = defaultConfigFile
	}
	_, err := toml.DecodeFile(file, &config)
	if err == nil {
		klog.Infof("using config file: %v", file)
		return nil
	}
	// When --config is not specified and defaultConfigFile is not found,
	// fall back to embedded config.
	if errors.Is(err, os.ErrNotExist) && defaultConfigFile == "" {
		klog.Info("using embedded config")
		_, err = toml.Decode(embeddedConfig, &config)
		if err != nil { // Should never happen.
			panic("invalid embedded config: " + err.Error())
		}
		return nil
	}
	// Otherwise, error out.
	return fmt.Errorf("can't parse config file %q: %w", file, err)
}
