package main

import (
	"errors"
	"time"

	v1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
)

type Config struct {
	Components    []string
	FromFile      string
	FromURL       string
	Limit         int
	OperatorImage string
	OutputFile    string
	OutputFormat  string
	Parallelism   int
	TimeLimit     time.Duration
	Verbose       bool
}

type ArtifactPod struct {
	ApiVersion string       `json:"apiVersion"`
	Items      []corev1.Pod `json:"items"`
}

type ScanResult struct {
	Tag   *v1.TagReference
	Path  string
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
