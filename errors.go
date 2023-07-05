package main

import "errors"

//go:generate go run gen_errors_map.go -out errors_map.go

// Well-known errors returned by scan.
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
