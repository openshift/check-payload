//go:build go1.21

package main

import "debug/elf"

func isPie(file *elf.File) (bool, error) {
	vals, err := file.DynValue(elf.DT_FLAGS_1)
	if err != nil {
		return false, err
	}
	for _, f := range vals {
		if elf.DynFlag1(f)&elf.DF_1_PIE != 0 {
			return true, nil
		}
	}
	return false, nil
}
