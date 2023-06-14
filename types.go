package main

import (
	"errors"
	"time"

	"github.com/carlmjohnson/versioninfo"
	v1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type Config struct {
	Components    []string      `json:"components"`
	Filter        []string      `json:"filter"`
	FromFile      string        `json:"from_file"`
	FromURL       string        `json:"from_url"`
	Limit         int           `json:"limit"`
	NodeScan      string        `json:"node_scan"`
	OperatorImage string        `json:"operator_image"`
	OutputFile    string        `json:"output_file"`
	OutputFormat  string        `json:"output_format"`
	Parallelism   int           `json:"parallelism"`
	TimeLimit     time.Duration `json:"time_limit"`
	Verbose       bool          `json:"verbose"`
}

func (c *Config) Log() {
	klog.InfoS("using config",
		"components", c.Components,
		"filter", c.Filter,
		"from_file", c.FromFile,
		"from_url", c.FromURL,
		"limit", c.Limit,
		"node_scan", c.NodeScan,
		"operator_image", c.OperatorImage,
		"output_file", c.OutputFile,
		"output_format", c.OutputFormat,
		"parallelism", c.Parallelism,
		"time_limit", c.TimeLimit,
		"verbose", c.Verbose,
		"version", versioninfo.Revision,
	)
}

type ArtifactPod struct {
	ApiVersion string       `json:"apiVersion"`
	Items      []corev1.Pod `json:"items"`
}

type ScanResult struct {
	Tag   *v1.TagReference
	Path  string
	Skip  bool
	Error error
}

type ScanResults struct {
	Items []*ScanResult
}

func NewScanResults() *ScanResults {
	return &ScanResults{}
}

func (sr *ScanResults) Append(result *ScanResult) *ScanResults {
	sr.Items = append(sr.Items, result)
	return sr
}

func NewScanResult() *ScanResult {
	return &ScanResult{}
}

func (r *ScanResult) Success() *ScanResult {
	r.Error = nil
	return r
}

func (r *ScanResult) Skipped() *ScanResult {
	r.Skip = true
	return r
}

func (r *ScanResult) SetOpenssl(info OpensslInfo) *ScanResult {
	if !info.Present {
		r.SetError(errors.New("openssl library not present"))
	} else if !info.FIPS {
		r.SetError(errors.New("openssl library is missing FIPS support"))
	}
	r.Path = info.Path
	return r
}

func (r *ScanResult) SetError(err error) *ScanResult {
	r.Error = err
	return r
}

func (r *ScanResult) SetPath(path string) *ScanResult {
	r.Path = path
	return r
}

func (r *ScanResult) SetBinaryPath(mountPath, path string) *ScanResult {
	r.Path = stripMountPath(mountPath, path)
	return r
}

func (r *ScanResult) SetTag(tag *v1.TagReference) *ScanResult {
	r.Tag = tag
	return r
}
