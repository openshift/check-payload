package types

import (
	"time"

	v1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
)

type Config struct {
	Components              []string      `json:"components"`
	FailOnWarnings          bool          `json:"fail_on_warnings"`
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
	Java                    bool          `json:"java"`
	PrintExceptions         bool          `json:"print_exceptions"`
	PullSecret              string        `json:"pull_secret"`
	TimeLimit               time.Duration `json:"time_limit"`
	Verbose                 bool          `json:"verbose"`
	UseRPMScan              bool          `json:"use_rpm_scan"`

	ConfigFile
}

// ConfigFile is a part of Config. It contains fields that can be set via a
// configuration files.
type ConfigFile struct {
	FilterFiles            []string `json:"filter_files" toml:"filter_files"`
	FilterDirs             []string `json:"filter_dirs" toml:"filter_dirs"`
	FilterImages           []string `json:"filter_images" toml:"filter_images"`
	JavaDisabledAlgorithms []string `json:"java_fips_disabled_algorithms" toml:"java_fips_disabled_algorithms"`

	PayloadIgnores map[string]IgnoreLists `toml:"payload"`
	TagIgnores     map[string]IgnoreLists `toml:"tag"`
	RPMIgnores     map[string]IgnoreLists `toml:"rpm"`
	ErrIgnores     ErrIgnoreList          `json:"ignore" toml:"ignore"`
}

type ErrIgnore struct {
	Error KnownError `toml:"error"`
	Files []string   `toml:"files"`
	Dirs  []string   `toml:"dirs"`
}

type ErrIgnoreList []ErrIgnore

type IgnoreLists struct {
	FilterFiles []string      `json:"filter_files" toml:"filter_files"`
	FilterDirs  []string      `json:"filter_dirs" toml:"filter_dirs"`
	ErrIgnores  ErrIgnoreList `json:"ignore" toml:"ignore"`
}

type ArtifactPod struct {
	APIVersion string       `json:"apiVersion"`
	Items      []corev1.Pod `json:"items"`
}

type ScanResult struct {
	Component *OpenshiftComponent
	Tag       *v1.TagReference
	RPM       string
	Path      string
	Skip      bool
	Error     *ValidationError
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

type JavaComponent struct {
	Entrypoint []string
	Cmd        []string
	WorkingDir string
}

type OpensslInfo struct {
	Present bool
	FIPS    bool
	Error   error
	Path    string
}

type ValidationError struct {
	Level ErrorLevel
	Error error
}

type ErrorLevel int64

const (
	Error ErrorLevel = iota
	Warning
)

func NewValidationError(err error) *ValidationError {
	if err == nil {
		return nil
	}
	return &ValidationError{Error: err, Level: Error}
}

func (ve *ValidationError) GetError() error {
	return ve.Error
}

func (ve *ValidationError) SetWarning() *ValidationError {
	ve.Level = Warning
	return ve
}

func (ve *ValidationError) IsError() bool {
	return ve.Error != nil && !ve.IsWarning()
}

func (ve *ValidationError) IsWarning() bool {
	return ve.Level == Warning
}
