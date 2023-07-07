package main

import (
	"time"

	v1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
)

type Config struct {
	ConfigFile              string        `json:"config_file"`
	Components              []string      `json:"components" toml:"components"`
	FilterFiles             []string      `json:"filter_files" toml:"filter_files"`
	FilterDirs              []string      `json:"filter_dirs" toml:"filter_dirs"`
	FilterImages            []string      `json:"filter_images" toml:"filter_images"`
	FilterFile              string        `json:"filter_file"`
	FromFile                string        `json:"from_file"`
	FromURL                 string        `json:"from_url"`
	InsecurePull            bool          `json:"insecure_pull"`
	Limit                   int           `json:"limit"`
	NodeScan                string        `json:"node_scan"`
	ContainerImageComponent string        `json:"container_image_component"`
	ContainerImage          string        `json:"container_image"`
	OutputFile              string        `json:"output_file"`
	OutputFormat            string        `json:"output_format"`
	Parallelism             int           `json:"parallelism"`
	PrintExceptions         bool          `json:"print_exceptions"`
	PullSecret              string        `json:"pull_secret"`
	TimeLimit               time.Duration `json:"time_limit"`
	Verbose                 bool          `json:"verbose"`

	PayloadIgnores map[string]IgnoreLists `toml:"payload"`
	TagIgnores     map[string]IgnoreLists `toml:"tag"`
	NodeIgnores    map[string]IgnoreLists `toml:"node"`
}

type IgnoreLists struct {
	FilterFiles []string `json:"filter_files" toml:"filter_files"`
	FilterDirs  []string `json:"filter_dirs" toml:"filter_dirs"`
}

type ArtifactPod struct {
	APIVersion string       `json:"apiVersion"`
	Items      []corev1.Pod `json:"items"`
}

type ScanResult struct {
	Component *OpenshiftComponent
	Tag       *v1.TagReference
	Path      string
	Skip      bool
	Error     error
}

type ScanResults struct {
	Items []*ScanResult
}

type OpenshiftComponent struct {
	Component           string `json:"component"`
	SourceLocation      string `json:"source_location"`
	MaintainerComponent string `json:"maintainer_component"`
	IsBundle            bool   `json:"is_bundle"`
}
