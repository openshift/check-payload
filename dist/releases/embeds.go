package releases

import (
	"embed"
	"fmt"
	"path/filepath"
)

//go:embed */*
var configs embed.FS

func GetConfigFor(version string) ([]byte, error) {
	bytes, err := configs.ReadFile(filepath.Join(version, "config.toml"))
	if err == nil {
		return bytes, nil
	}
	// Add a list of valid versions to the error message.
	dirs, err := configs.ReadDir(".")
	if err != nil { // Should not happen.
		return nil, fmt.Errorf("internal error: %w", err)
	}
	names := make([]string, len(dirs))
	for i, d := range dirs {
		names[i] = d.Name()
	}

	return nil, fmt.Errorf("embedded config for version %s is not available; use one of %+v", version, names)
}
