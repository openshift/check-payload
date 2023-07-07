package main

import (
	"errors"
	"time"

	v1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
)

type Config struct {
	IgnoreErrors            IgnoreErrors  `json:"ignore_error" toml:"ignore_errors"`
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
	RpmIgnores     map[string]IgnoreLists `toml:"rpm"`
}

type IgnoreError struct {
	Error KnownError `toml:"error"`
	Files []string   `toml:"files"`
}
type IgnoreErrors []IgnoreError

type IgnoreLists struct {
	FilterFiles  []string     `json:"filter_files" toml:"filter_files"`
	FilterDirs   []string     `json:"filter_dirs" toml:"filter_dirs"`
	IgnoreErrors IgnoreErrors `json:"ignore_errors" toml:"ignore_errors"`
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

// ForFile checks if an err should be ignored for a specified file.
func (i IgnoreErrors) ForFile(file string, err error) bool {
	if len(i) == 0 {
		return false
	}

	for _, ie := range i {
		if !errors.Is(err, ie.Error.Err) {
			continue
		}
		for _, f := range ie.Files {
			if file == f {
				return true
			}
		}
	}

	return false
}
