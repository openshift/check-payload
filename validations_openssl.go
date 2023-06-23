package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type OpensslInfo struct {
	Present bool
	FIPS    bool
	Error   error
	Path    string
}

func findLib(mountPath string, searchPaths []string, subname string) (path string, err error) {
	var returnPath string
	for _, path := range searchPaths {
		files, err := os.ReadDir(filepath.Join(mountPath, path))
		if err != nil {
			continue
		}
		for _, file := range files {
			if strings.Contains(file.Name(), subname) && !strings.Contains(file.Name(), "hmac") {
				returnPath = filepath.Join(path, file.Name())
				break
			}
		}
	}
	if returnPath == "" {
		return "", errors.New("openssl not found")
	}
	return returnPath, nil
}

func validateOpenssl(ctx context.Context, mountPath string) OpensslInfo {
	info := OpensslInfo{
		Present: false,
		FIPS:    false,
		Error:   nil,
	}

	path, err := findLib(mountPath, []string{"/usr/lib64", "/usr/lib"}, "libcrypto.so")
	if err != nil {
		info.Present = false
		info.FIPS = false
		return info
	}
	info.Path = path

	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "nm", "-D", filepath.Join(mountPath, path))
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		info.Error = err
		return info
	}

	info.Present = true
	info.FIPS = bytes.Contains(stdout.Bytes(), []byte("FIPS_mode")) || bytes.Contains(stdout.Bytes(), []byte("fips_mode"))

	return info
}
