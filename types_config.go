package main

import (
	"github.com/carlmjohnson/versioninfo"
	"k8s.io/klog/v2"
)

func (c *Config) Log() {
	klog.InfoS("using config",
		"components", c.Components,
		"filter_paths", c.FilterPaths,
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
