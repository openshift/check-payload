package main

import (
	"errors"

	v1 "github.com/openshift/api/image/v1"
)

func NewScanResult() *ScanResult {
	return &ScanResult{}
}

func (r *ScanResult) SetOperator(operator string) *ScanResult {
	r.OperatorName = operator
	return r
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
