package argon2

// Key is a stand-in used only to test reachability, not a crypto implementation.
//go:noinline
func Key(value []byte) []byte { return append([]byte{}, value...) }
