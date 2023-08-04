package releases

import (
	"embed"
	"fmt"
	"path/filepath"
	"sort"

	semver "github.com/Masterminds/semver/v3"
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
	sort.Slice(names, func(i, j int) bool {
		v1, _ := semver.NewVersion(names[i])
		v2, _ := semver.NewVersion(names[j])
		return v1.LessThan(v2)
	})
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
