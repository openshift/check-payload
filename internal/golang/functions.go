package golang

import (
	"bytes"
	"compress/gzip"
	"debug/buildinfo"
	"debug/gosym"
	"debug/macho"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
)

// Function identifies a retained Go function. Symbol preserves the original name,
// including vendor prefixes, receivers and compiler-generated suffixes.
type Function struct {
	Package string
	Symbol  string
}

// ReadFunctions extracts available identities without interpreting crypto policy.
// A nil slice with no error means the file is not a supported Go executable.
func ReadFunctions(path string) ([]Function, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	var magic [4]byte
	_, err = io.ReadFull(file, magic[:])
	file.Close()
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, nil
		}
		return nil, err
	}
	compressed := magic[0] == 0x1f && magic[1] == 0x8b
	if !compressed && magic != [4]byte{0x7f, 'E', 'L', 'F'} &&
		magic != [4]byte{0xcf, 0xfa, 0xed, 0xfe} && magic != [4]byte{0xce, 0xfa, 0xed, 0xfe} {
		return nil, nil
	}
	if compressed {
		unpacked, err := unpackExecutable(path)
		if err != nil {
			return nil, err
		}
		defer os.Remove(unpacked)
		path = unpacked
	}
	bi, err := buildinfo.ReadFile(path)
	if err != nil {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, readErr
		}
		if bytes.Contains(data, []byte("Go buildinf:")) || bytes.Contains(data, []byte("Go build ID:")) {
			return nil, fmt.Errorf("cannot read Go build information: %w", err)
		}
		return nil, nil
	}
	table, err := readFunctionTable(path, bi)
	if err != nil {
		return nil, err
	}
	functions := make([]Function, 0, len(table.Funcs))
	for _, fn := range table.Funcs {
		functions = append(functions, Function{Package: fn.PackageName(), Symbol: fn.Name})
	}
	sort.Slice(functions, func(i, j int) bool { return functions[i].Symbol < functions[j].Symbol })
	return functions, nil
}

func readFunctionTable(path string, bi *buildinfo.BuildInfo) (table *gosym.Table, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			table = nil
			err = fmt.Errorf("corrupt Go runtime function metadata: %v", recovered)
		}
	}()
	// ELF uses the same reader as the existing Go FIPS validations.
	if table, err = ReadTable(path, bi); err == nil {
		if table == nil || len(table.Funcs) == 0 {
			return nil, fmt.Errorf("empty Go runtime function metadata")
		}
		return table, nil
	}
	file, machoErr := macho.Open(path)
	if machoErr != nil {
		return nil, fmt.Errorf("cannot read Go runtime function metadata: %w", err)
	}
	defer file.Close()
	section, text := file.Section("__gopclntab"), file.Section("__text")
	if section == nil || text == nil {
		return nil, fmt.Errorf("Mach-O executable has no Go runtime function metadata")
	}
	data, err := section.Data()
	if err != nil {
		return nil, err
	}
	table, err = gosym.NewTable(nil, gosym.NewLineTable(data, text.Addr))
	if err != nil {
		return nil, err
	}
	if len(table.Funcs) == 0 {
		return nil, fmt.Errorf("empty Go runtime function metadata")
	}
	return table, nil
}

func unpackExecutable(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	zr, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	tmp, err := os.CreateTemp("", "check-payload-go-*")
	if err != nil {
		return "", err
	}
	const maxSize = 512 << 20
	n, copyErr := io.Copy(tmp, io.LimitReader(zr, maxSize+1))
	closeErr := tmp.Close()
	if err := errors.Join(copyErr, closeErr); err != nil || n > maxSize {
		os.Remove(tmp.Name())
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("decompressed executable exceeds %d bytes", maxSize)
	}
	return tmp.Name(), nil
}
