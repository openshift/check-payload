package validations

import (
	"context"
	"testing"
)

func TestEXE(t *testing.T) {
	if err := validateExe(context.Background(), "/usr/bin/lua5.1", nil); err != nil {
		t.Fatal(err)
	}
}
