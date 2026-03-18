package types

import "testing"

func TestVersionAtLeast(t *testing.T) {
	tests := []struct {
		installed string
		min       string
		want      bool
	}{
		{"3.0.1", "3.0.0", true},
		{"3.0.1-1.el9", "3.0.0", true},
		{"3.0.0", "3.0.0", true},
		{"2.9.0", "3.0.0", false},
		{"", "3.0.0", false},
		{"3.0.0", "3.0.1", false},
		{"3.0.7", "3.0.7", true},
		{"3.0.7-1.el9_4", "3.0.7", true},
	}
	for _, tt := range tests {
		if got := VersionAtLeast(tt.installed, tt.min); got != tt.want {
			t.Errorf("VersionAtLeast(%q, %q) = %v, want %v", tt.installed, tt.min, got, tt.want)
		}
	}
}
