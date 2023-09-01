package releases

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	semver "github.com/Masterminds/semver/v3"
)

//go:embed */*
var configs embed.FS

const JavaFips = "FIPS.java"

// GetVersions returns the list of versions for those embedded configs
// are available.
func GetVersions() []string {
	dirs, err := configs.ReadDir(".")
	if err != nil { // Should not happen.
		return nil
	}
	names := make([]string, 0, len(dirs))
	for _, d := range dirs {
		if name := d.Name(); name != "java" {
			names = append(names, name)
		}
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

// GetJavaFile returns the java source file.
func GetJavaFile() (fileFullPath, fileName string, err error) {
	bytes, err := configs.ReadFile("java/" + JavaFips)
	if err != nil {
		return "", "", err
	}

	return createTempFile(bytes, "fipsJava")
}

// GetAlgorithmFile returns a file with disabled algorithms to check.
func GetAlgorithmFile(javaDisabledAlgorithms []string) (fileFullPath, fileName string, err error) {
	return createTempFile([]byte(strings.Join(javaDisabledAlgorithms, "\n")), "disabledAlgorithms")
}

// createTempFile returns a temporary file with specified content and prefix, created in the OS's default temp folder.
// Closing of the file (defer os.Remove(file.Name())) is the caller's responsibility.
func createTempFile(content []byte, prefix string) (fileFullPath, fileName string, err error) {
	file, err := os.CreateTemp("", prefix)
	if err != nil {
		return "", "", err
	}
	defer func() {
		if err != nil {
			os.Remove(file.Name())
		}
	}()
	fileModeRO := os.FileMode(syscall.S_IRUSR | syscall.S_IRGRP | syscall.S_IROTH)
	if err = file.Chmod(fileModeRO); err != nil {
		return "", "", err
	}
	_, err = file.Write(content)
	if err != nil {
		return "", "", err
	}

	return file.Name(), filepath.Base(file.Name()), nil
}
