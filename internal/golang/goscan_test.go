package golang

import (
	"debug/buildinfo"
	"debug/elf"
	"encoding/binary"
	"testing"
)

func TestFindPclntab_ValidHeader(t *testing.T) {
	magic := []byte{0xf1, 0xff, 0xff, 0xff}
	data := make([]byte, 16)
	copy(data[0:4], magic)
	data[4] = 0
	data[5] = 0
	data[6] = 1
	data[7] = 8

	idx := findPclntab(data, magic)
	if idx != 0 {
		t.Fatalf("expected index 0, got %d", idx)
	}
}

func TestFindPclntab_SkipsFalseMatch(t *testing.T) {
	magic := []byte{0xff, 0xff, 0xff, 0xf1} // BE magic
	data := make([]byte, 32)

	// False match at offset 0: magic present but invalid header bytes
	copy(data[0:4], magic)
	data[4] = 0x42
	data[5] = 0x00
	data[6] = 1
	data[7] = 8

	// Real match at offset 16
	copy(data[16:20], magic)
	data[20] = 0
	data[21] = 0
	data[22] = 4
	data[23] = 8

	idx := findPclntab(data, magic)
	if idx != 16 {
		t.Fatalf("expected index 16 (skip false match), got %d", idx)
	}
}

func TestFindPclntab_NoMatch(t *testing.T) {
	magic := []byte{0xf1, 0xff, 0xff, 0xff}
	data := make([]byte, 64)

	idx := findPclntab(data, magic)
	if idx != -1 {
		t.Fatalf("expected -1, got %d", idx)
	}
}

func TestFindPclntab_AllFalseMatches(t *testing.T) {
	magic := []byte{0xff, 0xff, 0xff, 0xf1}
	data := make([]byte, 32)

	copy(data[0:4], magic)
	data[4] = 0xff
	data[5] = 0xff
	data[6] = 3 // invalid quantum
	data[7] = 8

	copy(data[16:20], magic)
	data[20] = 0
	data[21] = 0
	data[22] = 7 // invalid quantum
	data[23] = 8

	idx := findPclntab(data, magic)
	if idx != -1 {
		t.Fatalf("expected -1 (all false matches), got %d", idx)
	}
}

func TestFindPclntab_BEMagicValidQuantumAndPtrSize(t *testing.T) {
	for _, quantum := range []byte{1, 2, 4} {
		for _, ptrsize := range []byte{4, 8} {
			magic := make([]byte, 4)
			binary.BigEndian.PutUint32(magic, go120magic)
			data := make([]byte, 16)
			copy(data[0:4], magic)
			data[4] = 0
			data[5] = 0
			data[6] = quantum
			data[7] = ptrsize

			idx := findPclntab(data, magic)
			if idx != 0 {
				t.Fatalf("quantum=%d ptrsize=%d: expected 0, got %d", quantum, ptrsize, idx)
			}
		}
	}
}

func TestReadTable(t *testing.T) {
	tests := []struct {
		name           string
		fixture        string
		wantSection    string // expected ELF section containing pclntab
		rejectSections []string
		wantMachine    elf.Machine
	}{
		{
			"non-PIE amd64 boringcrypto", "../../test/resources/fips_compliant_app",
			".gopclntab", nil, elf.EM_X86_64,
		},
		// Regression: LE magic 0xF1FFFFFF matches inside .gopclntab data
		// before the real BE pclntab at offset 0.
		{
			"Go1.24 s390x CGO (false LE match)", "../../test/resources/go124_s390x_app",
			".gopclntab", nil, elf.EM_S390,
		},
		// .data.rel.ro.gopclntab section layout from internal PIE
		{
			"Go1.24 amd64 internal PIE", "../../test/resources/go124_internal_pie_amd64_app",
			".data.rel.ro.gopclntab", nil, elf.EM_X86_64,
		},
		// .gopclntab as separate section in Go 1.26 PIE (#329)
		{
			"Go1.26 s390x PIE", "../../test/resources/pie_go126_s390x_app",
			".gopclntab", nil, elf.EM_S390,
		},
		// quantum=4 unique to ppc64le (little-endian PowerPC)
		{
			"Go1.24 ppc64le CGO (quantum=4 LE)", "../../test/resources/go124_ppc64le_app",
			".gopclntab", nil, elf.EM_PPC64,
		},
		// .data.rel.ro fallback: pclntab not in a dedicated section,
		// exercises the magic scanning loop in ReadTable.
		{
			"Go1.24 amd64 external PIE (.data.rel.ro)", "../../test/resources/go124_external_pie_amd64_app",
			".data.rel.ro",
			[]string{".gopclntab", ".data.rel.ro.gopclntab"},
			elf.EM_X86_64,
		},
		// native Go FIPS module (crypto/fips140)
		{
			"Go native FIPS amd64", "../../test/resources/go-native-fips-app",
			".gopclntab", nil, elf.EM_X86_64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertFixtureInvariants(t, tt.fixture, tt.wantSection, tt.rejectSections, tt.wantMachine)

			bi, err := buildinfo.ReadFile(tt.fixture)
			if err != nil {
				t.Fatalf("buildinfo.ReadFile(%s): %v", tt.fixture, err)
			}

			table, err := ReadTable(tt.fixture, bi)
			if err != nil {
				t.Fatalf("ReadTable(%s): %v", tt.fixture, err)
			}
			if table == nil {
				t.Fatal("ReadTable returned nil table")
			}
			if len(table.Funcs) == 0 {
				t.Fatal("ReadTable returned 0 functions")
			}
		})
	}
}

func assertFixtureInvariants(t *testing.T, path string, wantSection string, rejectSections []string, wantMachine elf.Machine) {
	t.Helper()
	exe, err := elf.Open(path)
	if err != nil {
		t.Fatalf("elf.Open(%s): %v", path, err)
	}
	defer exe.Close()

	if exe.Machine != wantMachine {
		t.Fatalf("fixture arch = %v, want %v", exe.Machine, wantMachine)
	}

	if exe.Section(wantSection) == nil {
		t.Fatalf("fixture missing expected section %s", wantSection)
	}

	for _, name := range rejectSections {
		if exe.Section(name) != nil {
			t.Fatalf("fixture has section %s which should be absent (would bypass the %s fallback path)", name, wantSection)
		}
	}
}
