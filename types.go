package main

import (
	"time"

	v1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
)

type Config struct {
	Components              []string      `json:"components"`
	Filter                  []string      `json:"filter"`
	FromFile                string        `json:"from_file"`
	FromURL                 string        `json:"from_url"`
	Limit                   int           `json:"limit"`
	NodeScan                string        `json:"node_scan"`
	ContainerImageComponent string        `json:"container_image_component"`
	ContainerImage          string        `json:"container_image"`
	OutputFile              string        `json:"output_file"`
	OutputFormat            string        `json:"output_format"`
	Parallelism             int           `json:"parallelism"`
	TimeLimit               time.Duration `json:"time_limit"`
	Verbose                 bool          `json:"verbose"`
}

type ArtifactPod struct {
	ApiVersion string       `json:"apiVersion"`
	Items      []corev1.Pod `json:"items"`
}

type ScanResult struct {
	OperatorName string
	Tag          *v1.TagReference
	Path         string
	Skip         bool
	Error        error
}

type ScanResults struct {
	Items []*ScanResult
}
