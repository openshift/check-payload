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
		"payload_ignores", c.PayloadIgnores,
		"output_file", c.OutputFile,
		"output_format", c.OutputFormat,
		"parallelism", c.Parallelism,
		"time_limit", c.TimeLimit,
		"verbose", c.Verbose,
		"version", versioninfo.Revision,
	)
}

func isFileMatch(path string, filterFiles []string) bool {
	for _, f := range filterFiles {
		if f == path {
			return true
		}
	}
	return false
}

func isDirMatch(path string, filterDirs []string) bool {
	for _, f := range filterDirs {
		if f == path {
			return true
		}
	}
	return false
}

func isDirPrefixMatch(path string, filterDirs []string) bool {
	for _, dir := range filterDirs {
		if strings.HasPrefix(path, dir+"/") {
			return true
		}
	}
	return false
}

func (c *Config) isFileIgnoredByComponent(path string, component *OpenshiftComponent) bool {
	if component == nil {
		return false
	}
	if op, ok := c.PayloadIgnores[component.Component]; ok {
		return isFileMatch(path, op.FilterFiles)
	}
	return false
}

func (c *Config) isDirIgnoredByComponent(path string, component *OpenshiftComponent) bool {
	if component == nil {
		return false
	}
	if op, ok := c.PayloadIgnores[component.Component]; ok {
		return isFileMatch(path, op.FilterDirs)
	}
	return false
}

func (c *Config) IgnoreFile(path string) bool {
	return isFileMatch(path, c.FilterFiles)
}

func (c *Config) IgnoreFileWithComponent(path string, component *OpenshiftComponent) bool {
	return c.isFileIgnoredByComponent(path, component) || c.IgnoreFile(path)
}

func (c *Config) IgnoreDir(path string) bool {
	return isFileMatch(path, c.FilterDirs)
}

func (c *Config) IgnoreDirWithComponent(path string, component *OpenshiftComponent) bool {
	return c.isDirIgnoredByComponent(path, component) || c.IgnoreDir(path)
}

func (c *Config) IgnoreDirPrefix(path string) bool {
	return isDirMatch(path, c.FilterDirs)
}
