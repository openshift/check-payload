package golang

import (
	"bytes"
	"debug/buildinfo"
	"debug/elf"
	"debug/gosym"
	"encoding/binary"
	"fmt"
	"log/slog"
	"strings"
)

// from https://gitlab.cee.redhat.com/dbenoit/gosyms-example/-/blob/master/gosyms-example.go

// From go/src/debug/gosym/pclntab.go
const (
	go12magic  = 0xfffffffb
	go116magic = 0xfffffffa
	go118magic = 0xfffffff0
	go120magic = 0xfffffff1
)

// Select the magic number based on the Go version
func magicNumber(goVersion string) []byte {
	bs := make([]byte, 4)
	magic := _magicNumber(goVersion)
	binary.LittleEndian.PutUint32(bs, magic)
	return bs
}

// Select the magic number based on the Go version
func magicNumberBigEndian(goVersion string) []byte {
	bs := make([]byte, 4)
	magic := _magicNumber(goVersion)
	binary.BigEndian.PutUint32(bs, magic)
	return bs
}

func _magicNumber(goVersion string) uint32 {
	var magic uint32
	if strings.Compare(goVersion, "go1.20") >= 0 {
		magic = go120magic
	} else if strings.Compare(goVersion, "go1.18") >= 0 {
		magic = go118magic
	} else if strings.Compare(goVersion, "go1.16") >= 0 {
		magic = go116magic
	} else {
		magic = go12magic
	}
	return magic
}

// Header layout: magic(4) + two zero bytes + quantum(1,2,4) + ptrsize(4,8).
func isValidPclntabHeader(data []byte) bool {
	return len(data) >= 8 &&
		data[4] == 0 && data[5] == 0 &&
		(data[6] == 1 || data[6] == 2 || data[6] == 4) &&
		(data[7] == 4 || data[7] == 8)
}

func findPclntab(data []byte, magic []byte) int {
	for offset := 0; offset+8 <= len(data); {
		idx := bytes.Index(data[offset:], magic)
		if idx < 0 {
			return -1
		}
		candidate := offset + idx
		if candidate+8 > len(data) {
			return -1
		}
		if isValidPclntabHeader(data[candidate:]) {
			return candidate
		}
		offset = candidate + 1
	}
	return -1
}

func tryParseTable(tableData []byte, offset int, textAddr uint64) *gosym.Table {
	lineTable := gosym.NewLineTable(tableData[offset:], textAddr)
	symTable, err := gosym.NewTable([]byte{}, lineTable)
	if err != nil {
		slog.Debug("pclntab candidate rejected", "offset", offset, "error", err)
		return nil
	}
	if len(symTable.Funcs) == 0 {
		slog.Debug("pclntab candidate rejected", "offset", offset, "reason", "zero functions")
		return nil
	}
	return symTable
}

// ReadTable opens a Go ELF executable and reads its symbol table from the pclntab.
func ReadTable(fileName string, bi *buildinfo.BuildInfo) (*gosym.Table, error) {
	exe, err := elf.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer exe.Close()

	textSection := exe.Section(".text")
	if textSection == nil {
		return nil, fmt.Errorf("missing .text section in %s", fileName)
	}

	// The pclntab lives in different ELF sections depending on Go version
	// and link mode:
	//   .gopclntab              - non-PIE, or Go 1.26+ PIE
	//   .data.rel.ro.gopclntab  - Go <= 1.25 internal PIE (CGO_ENABLED=0)
	//   .data.rel.ro            - Go <= 1.25 external PIE (CGO_ENABLED=1)
	var section *elf.Section
	for _, name := range []string{".gopclntab", ".data.rel.ro.gopclntab", ".data.rel.ro"} {
		if s := exe.Section(name); s != nil {
			section = s
			break
		}
	}
	if section == nil {
		return nil, fmt.Errorf("could not find pclntab section in %s", fileName)
	}
	tableData, err := section.Data()
	if err != nil {
		return nil, fmt.Errorf("could not read %s section from %s", section.Name, fileName)
	}

	// Dedicated pclntab sections start at offset 0; skip magic scanning.
	if section.Name != ".data.rel.ro" && isValidPclntabHeader(tableData) {
		if table := tryParseTable(tableData, 0, textSection.Addr); table != nil {
			return table, nil
		}
	}

	// Try native endianness first to reduce false magic matches.
	magics := [][]byte{magicNumber(bi.GoVersion), magicNumberBigEndian(bi.GoVersion)}
	if exe.ByteOrder == binary.BigEndian {
		magics[0], magics[1] = magics[1], magics[0]
	}

	for _, magic := range magics {
		for offset := 0; ; {
			idx := findPclntab(tableData[offset:], magic)
			if idx < 0 {
				break
			}
			candidate := offset + idx
			if table := tryParseTable(tableData, candidate, textSection.Addr); table != nil {
				return table, nil
			}
			offset = candidate + 1
		}
	}

	return nil, fmt.Errorf("could not find valid pclntab in %s (section=%s)", fileName, section.Name)
}

// ExpectedSyms checks that .gopclntab contains any of the expectedSymbols.
func ExpectedSyms(expectedSymbols []string, symTable *gosym.Table) bool {
	for _, s := range expectedSymbols {
		if symTable.LookupFunc(s) != nil {
			return true
		}
	}
	return false
}
