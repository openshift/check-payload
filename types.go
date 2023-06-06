package main

import (
	"time"

	v1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
)

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

type Config struct {
	FromURL      string
	FromFile     string
	Limit        int
	TimeLimit    time.Duration
	Parallelism  int
	OutputFormat string
	OutputFile   string
	Components   []string
}

func NewScanResult() *ScanResult {
	return &ScanResult{}
}

func (r *ScanResult) Success() *ScanResult {
	r.Error = nil
	return r
}

func (r *ScanResult) SetError(err error) *ScanResult {
	r.Error = err
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
