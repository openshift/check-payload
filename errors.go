package main

//go:generate go run gen_errors_map.go -out errors_map.go

import (
	"errors"
	"fmt"
)

// Well-known errors returned by scan. These names can be specified in
// ignore_errors configuration sections.
var (
	ErrGoInvalidTag      = errors.New("go binary has invalid build tag(s) set")
	ErrGoMissingSymbols  = errors.New("go binary does not contain required symbol(s)")
	ErrGoMissingTag      = errors.New("go binary does not contain required tag(s)")
	ErrMultipleLibcrypto = errors.New("openssl: found multiple different libcrypto versions")
	ErrNoCgoInit         = errors.New("x_cgo_init not found")
	ErrNoLibcrypto       = errors.New("openssl: did not find libcrypto library within binary")
	ErrNoLibcryptoSO     = errors.New("could not find dependent openssl version within container image")
	ErrNotCgoEnabled     = errors.New("go binary is not CGO_ENABLED")
	ErrNotDynLinked      = errors.New("executable is not dynamically linked")
)

// KnownError is a type used to parse "error = Err*" values in toml config.
type KnownError struct {
	Err error
	Str string
}

// UnmarshalText is used when parsing toml config.
func (e *KnownError) UnmarshalText(text []byte) error {
	str := string(text)
	if err, ok := KnownErrors[str]; ok {
		e.Str = str
		e.Err = err
		return nil
	}
	return fmt.Errorf("error=%q is not recognized in config; run \"show errors\" to see known errors", str)
}

// String is used when printing the current configuration.
func (e KnownError) String() string {
	return e.Str
}
