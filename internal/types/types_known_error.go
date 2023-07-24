package types

import "fmt"

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
	return fmt.Errorf("error=%q is not recognized in config", str)
}

// String is used when printing the current configuration.
func (e KnownError) String() string {
	return e.Str
}
