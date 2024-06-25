package types

import (
	"errors"

	v1 "github.com/openshift/api/image/v1"
)

func NewScanResult() *ScanResult {
	return &ScanResult{}
}

func (r *ScanResult) SetComponent(component *OpenshiftComponent) *ScanResult {
	r.Component = component
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

func (r *ScanResult) IsLevel(level ErrorLevel) bool {
	return r.Error != nil && r.Error.Level == level
}

func (r *ScanResult) IsSuccess() bool {
	return r.Error == nil
}

func (r *ScanResult) Status() string {
	if r.Error == nil {
		return "success"
	}
	switch r.Error.Level {
	case Error:
		return "failed"
	case Warning:
		return "warning"
	}
	// Should never happen.
	return "<unknown>"
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

func (r *ScanResult) SetOS(info OSInfo) *ScanResult {
	if info.Error != nil {
		r.SetValidationError(info.Error)
	} else if !info.Certified {
		r.SetError(ErrOSNotCertified)
	}

	// We currently only support checking a specific file for the
	// distribution information. This may need to evolve to be more
	// flexible in the future if distribution detection becomes more
	// advanced.
	r.Path = info.Path
	return r
}

func (r *ScanResult) SetValidationError(err *ValidationError) *ScanResult {
	r.Error = err
	return r
}

func (r *ScanResult) SetError(err error) *ScanResult {
	r.Error = NewValidationError(err)
	return r
}

func (r *ScanResult) SetPath(path string) *ScanResult {
	r.Path = path
	return r
}

func (r *ScanResult) SetTag(tag *v1.TagReference) *ScanResult {
	r.Tag = tag
	return r
}

func (r *ScanResult) SetRPM(rpm string) *ScanResult {
	r.RPM = rpm
	return r
}
