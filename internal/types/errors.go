package types

import "errors"

//go:generate go run gen_error_map.go -out error_map.go

// Well-known errors returned by scan. If you modify this list,
// do not forget to run 'go generate'.
var (
	ErrGoInvalidTag                = errors.New("go binary has invalid build tag(s) set")
	ErrGoMissingSymbols            = errors.New("go binary does not contain required symbol(s)")
	ErrGoMissingTag                = errors.New("go binary does not contain required tag(s)")
	ErrGoNoCgoInit                 = errors.New("x_cgo_init or _cgo_topofstack not found")
	ErrGoNoTags                    = errors.New("go binary has no build tags set (should have strictfipsruntime)")
	ErrGoNotCgoEnabled             = errors.New("go binary is not CGO_ENABLED")
	ErrGoNotGoExperiment           = errors.New("go binary does not enable GOEXPERIMENT=strictfipsruntime")
	ErrLibcryptoMany               = errors.New("openssl: found multiple different libcrypto versions")
	ErrLibcryptoMissing            = errors.New("openssl: did not find libcrypto library within binary")
	ErrLibcryptoSoMissing          = errors.New("could not find dependent openssl version within container image")
	ErrNotDynLinked                = errors.New("executable is not dynamically linked")
	ErrOSNotCertified              = errors.New("operating system is not FIPS certified")
	ErrDistributionFileMissing     = errors.New("could not find distribution file")
	ErrCertifiedDistributionsEmpty = errors.New("certified_distributions is empty, consider using -V [VERSION] for check-payload")
	ErrDetectedExcludedModule      = errors.New("detected a library that is incompatible with FIPS, check to make sure it is not performing any cryptographic operations")
)
