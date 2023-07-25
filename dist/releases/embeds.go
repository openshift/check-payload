package releases

import (
	"embed"
	"fmt"
	"path/filepath"
)

//go:embed */*
var configs embed.FS

// GetVersions returns the list of versions for those embedded configs
// are available.
func GetVersions() []string {
	dirs, err := configs.ReadDir(".")
	if err != nil { // Should not happen.
		return nil
	}
	names := make([]string, len(dirs))
	for i, d := range dirs {
		names[i] = d.Name()
	}

	return names
}

// GetConfigFor returns the configuration for a given version, if found.
func GetConfigFor(version string) ([]byte, error) {
	bytes, err := configs.ReadFile(filepath.Join(version, "config.toml"))
	if err == nil {
		return bytes, nil
	}

	return nil, fmt.Errorf("embedded config for version %s is not available; use one of %+v", version, GetVersions())
}
