package main

import (
	"strings"

	"github.com/carlmjohnson/versioninfo"
	"k8s.io/klog/v2"
)

func (c *Config) Log() {
	klog.InfoS("using config",
		"components", c.Components,
		"filter_dirs", c.FilterDirs,
		"filter_files", c.FilterFiles,
		"filter_images", c.FilterImages,
		"from_file", c.FromFile,
		"from_url", c.FromURL,
		"limit", c.Limit,
		"node_scan", c.NodeScan,
		"container_image", c.ContainerImage,
		"output_file", c.OutputFile,
		"output_format", c.OutputFormat,
		"parallelism", c.Parallelism,
		"time_limit", c.TimeLimit,
		"verbose", c.Verbose,
		"version", versioninfo.Revision,
	)
}

func (c *Config) IgnoreFile(path string) bool {
	for _, f := range c.FilterFiles {
		if f == path {
			return true
		}
	}

	return false
}

func (c *Config) IgnoreDir(path string) bool {
	for _, f := range c.FilterDirs {
		if f == path {
			return true
		}
	}

	return false
}

func (c *Config) IgnoreDirPrefix(path string) bool {
	for _, dir := range c.FilterDirs {
		if strings.HasPrefix(path, dir+"/") {
			return true
		}
	}

	return false
}
