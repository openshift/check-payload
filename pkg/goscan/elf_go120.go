//go:build !go1.21

package goscan

import "debug/elf"

func IsPie(file *elf.File) (bool, error) {
	vals, err := dynValue(file, elf.DT_FLAGS_1)
	if err != nil {
		return false, err
	}
	for _, f := range vals {
		if dynFlag1(f)&DF_1_PIE != 0 {
			return true, nil
		}
	}
	return false, nil
}

// Below code is copied from go1.21rc2's src/debug/elf. It was added there
// by https://go.dev/cl/452617 and https://go.dev/cl/452496.

type dynFlag1 uint32

const DF_1_PIE dynFlag1 = 0x08000000 //nolint:revive // this comes from C world.

// dynValue returns the values listed for the given tag in the file's dynamic
// section. Copied from go1.21rc2
func dynValue(f *elf.File, tag elf.DynTag) ([]uint64, error) {
	ds := f.SectionByType(elf.SHT_DYNAMIC)
	if ds == nil {
		return nil, nil
	}
	d, err := ds.Data()
	if err != nil {
		return nil, err
	}

	// Parse the .dynamic section as a string of bytes.
	var vals []uint64
	for len(d) > 0 {
		var t elf.DynTag
		var v uint64
		switch f.Class {
		case elf.ELFCLASS32:
			t = elf.DynTag(f.ByteOrder.Uint32(d[0:4]))
			v = uint64(f.ByteOrder.Uint32(d[4:8]))
			d = d[8:]
		case elf.ELFCLASS64:
			t = elf.DynTag(f.ByteOrder.Uint64(d[0:8]))
			v = f.ByteOrder.Uint64(d[8:16])
			d = d[16:]
		}
		if t == tag {
			vals = append(vals, v)
		}
	}
	return vals, nil
}
