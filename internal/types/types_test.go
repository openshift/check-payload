package types

import "testing"

func TestVersionInRange(t *testing.T) {
	tests := []struct {
		name      string
		installed string
		min, max  string
		wantMin   bool
		wantMax   bool
	}{
		{"above min", "3.0.1", "3.0.0", "", true, true},
		{"rpm suffix above min", "3.0.1-1.el9", "3.0.0", "", true, true},
		{"equal to min", "3.0.0", "3.0.0", "", true, true},
		{"below min", "2.9.0", "3.0.0", "", false, true},
		{"empty installed", "", "3.0.0", "", false, false},
		{"below min exact", "3.0.0", "3.0.1", "", false, true},
		{"rpm suffix equal", "3.0.7-1.el9_4", "3.0.7", "", true, true},
		{"v-prefix equal", "v1.0.0", "v1.0.0", "", true, true},
		{"v-prefix with hash", "v1.0.0-c2097c7c", "v1.0.0", "", true, true},

		{"at max", "1.1.1", "", "1.1.1", true, true},
		{"below max", "1.0.0", "", "1.1.1", true, true},
		{"rpm suffix at max", "1.1.1-72.el8_8", "", "1.1.1", true, true},
		{"above max", "3.0.7-1.el9_4", "", "1.1.1", true, false},
		{"above max no suffix", "3.0.7", "", "1.1.1", true, false},
		{"above max by patch", "3.0.8", "", "3.0.7", true, false},

		{"in range", "1.1.1-72.el8_8", "1.1.1", "1.1.1", true, true},
		{"below range", "1.0.0", "1.1.1", "1.1.1", false, true},
		{"above range", "3.0.7", "1.1.1", "1.1.1", true, false},
		{"no bounds", "3.0.7", "", "", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMin, gotMax := VersionInRange(tt.installed, tt.min, tt.max)
			if gotMin != tt.wantMin || gotMax != tt.wantMax {
				t.Errorf("VersionInRange(%q, %q, %q) = (%v, %v), want (%v, %v)",
					tt.installed, tt.min, tt.max, gotMin, gotMax, tt.wantMin, tt.wantMax)
			}
		})
	}
}
