package golang

import (
	"debug/buildinfo"
	"testing"
)

func TestReadTable_NonPIE(t *testing.T) {
	const fixture = "../../test/resources/fips_compliant_app"

	bi, err := buildinfo.ReadFile(fixture)
	if err != nil {
		t.Fatalf("buildinfo.ReadFile(%s): %v", fixture, err)
	}

	table, err := ReadTable(fixture, bi)
	if err != nil {
		t.Fatalf("ReadTable(%s): %v", fixture, err)
	}
	if table == nil {
		t.Fatal("ReadTable returned nil table")
	}
}

// TestReadTable_PIE_Go126_s390x exercises the section lookup fix from issue #329.
// Go 1.26 emits .gopclntab as a separate section even for PIE builds.
// Before the fix, ReadTable skipped .gopclntab when it saw -buildmode=pie
// and looked only in .data.rel.ro, which no longer contains the pclntab.
// On s390x (big-endian) .data.rel.ro has no accidental magic byte matches,
// so the old code fails deterministically — making this a true regression test.
func TestReadTable_PIE_Go126_s390x(t *testing.T) {
	const fixture = "../../test/resources/pie_go126_s390x_app"

	bi, err := buildinfo.ReadFile(fixture)
	if err != nil {
		t.Fatalf("buildinfo.ReadFile(%s): %v", fixture, err)
	}
	if bi.GoVersion == "" {
		t.Fatal("GoVersion is empty in build info")
	}

	table, err := ReadTable(fixture, bi)
	if err != nil {
		t.Fatalf("ReadTable(%s): %v", fixture, err)
	}
	if table == nil {
		t.Fatal("ReadTable returned nil table")
	}
}
