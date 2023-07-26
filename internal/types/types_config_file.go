package types

import (
	"go.uber.org/multierr"
)

func (c *ConfigFile) Add(add *ConfigFile) error {
	var err error

	c.FilterFiles = appendUniq("filter_files", &err, c.FilterFiles, add.FilterFiles)
	c.FilterDirs = appendUniq("filter_dirs", &err, c.FilterDirs, add.FilterDirs)
	c.FilterImages = appendUniq("filter_images", &err, c.FilterImages, add.FilterImages)

	c.PayloadIgnores = mergeLists("payload", &err, c.PayloadIgnores, add.PayloadIgnores)
	c.TagIgnores = mergeLists("tag", &err, c.TagIgnores, add.TagIgnores)
	c.RPMIgnores = mergeLists("rpm", &err, c.RPMIgnores, add.RPMIgnores)

	c.ErrIgnores = append(c.ErrIgnores, add.ErrIgnores...)

	return err
}

type errDup struct {
	Listname string
	Dup      string
}

func (e *errDup) Error() string {
	return "main config " + e.Listname + " already contains " + e.Dup
}

func contains(list []string, elem string) bool {
	for _, e := range list {
		if e == elem {
			return true
		}
	}
	return false
}

func appendUniq(listname string, perr *error, main, add []string) []string {
	for _, a := range add {
		if contains(main, a) {
			multierr.AppendInto(perr, &errDup{listname, a})
			continue
		}
		main = append(main, a)
	}

	return main
}

func mergeLists(name string, perr *error, main, add map[string]IgnoreLists) map[string]IgnoreLists {
	if main == nil {
		return add
	}

	for k, v := range add {
		if l, ok := main[k]; ok {
			keyname := "[" + name + "." + k + "]"
			l.FilterFiles = appendUniq(keyname+".filter_files", perr, l.FilterFiles, v.FilterFiles)
			l.FilterDirs = appendUniq(keyname+".filter_dirs", perr, l.FilterDirs, v.FilterDirs)
			l.ErrIgnores = append(l.ErrIgnores, v.ErrIgnores...)
			main[k] = l
		} else {
			main[k] = v
		}
	}
	return main
}
