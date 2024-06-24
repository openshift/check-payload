package types

import (
	"path/filepath"
	"strings"

	"go.uber.org/multierr"
)

// Validate validates the configuration. Currently it checks that
// all the file and directory paths are absolute and clean, and that
// there are no overlaps between each entry files and dirs.
// It returns errors and warnings; errors are considered fatal,
// while warnings are more like FYI.
func (c *ConfigFile) Validate() (err, warn error) {
	validateFileList("filter_files", &err, c.FilterFiles)
	validateFileList("filter_dirs", &err, c.FilterDirs)
	validateOverlaps("filter_", &warn, c.FilterFiles, c.FilterDirs)

	validateIgnoreLists("payload", &err, &warn, c.PayloadIgnores)
	validateIgnoreLists("tag", &err, &warn, c.TagIgnores)
	validateIgnoreLists("rpm", &err, &warn, c.RPMIgnores)

	validateErrIgnores("[[ignore]]", &err, &warn, c.ErrIgnores)

	return
}

type errBadPath struct {
	Listname  string
	Path      string
	CleanPath string
}

func (e *errBadPath) Error() string {
	return `config entry ` + e.Listname + ` contains unclean path "` + e.Path + `" (should have been "` + e.CleanPath + `")`
}

type errNAbsPath struct {
	Listname string
	Path     string
}

func (e *errNAbsPath) Error() string {
	return `config entry ` + e.Listname + ` contains non-absolute path "` + e.Path + `"`
}

type errOverlap struct {
	Listname string
	Path     string
	By       string
}

func (e *errOverlap) Error() string {
	return `config entry ` + e.Listname + ` contains a redundant path "` + e.Path + `", overlapped by "` + e.By + `"`
}

type errEmpty struct {
	Listname string
	What     string
}

func (e *errEmpty) Error() string {
	return `config entry ` + e.Listname + ` has no ` + e.What + ` set`
}

// validateFileList checks that the paths in the list are clean and absolute.
func validateFileList(listname string, perr *error, list []string) {
	for _, f := range list {
		cf := filepath.Clean(f)
		if f != cf {
			multierr.AppendInto(perr, &errBadPath{listname, f, cf})
		}
		if f[0] != '/' {
			multierr.AppendInto(perr, &errNAbsPath{listname, f})
		}
	}
}

func validateIgnoreLists(listname string, perr, pwarn *error, list map[string]IgnoreLists) {
	for k, v := range list {
		prefix := "[" + listname + "." + k
		validateFileList(prefix+"].filter_files", perr, v.FilterFiles)
		validateFileList(prefix+"].filter_dirs", perr, v.FilterDirs)
		validateOverlaps(prefix+"].filter_", pwarn, v.FilterFiles, v.FilterDirs)
		validateErrIgnores("["+prefix+".ignore]]", perr, pwarn, v.ErrIgnores)
	}
}

func validateErrIgnores(section string, perr, pwarn *error, l ErrIgnoreList) {
	for _, v := range l {
		// Make sure error is set.
		if v.Error.Str == "" {
			multierr.AppendInto(perr, &errEmpty{section, "error="})
		}
		// Make sure files/dirs/tags are not empty.
		if len(v.Files)+len(v.Dirs)+len(v.Tags) == 0 {
			multierr.AppendInto(perr, &errEmpty{section, "files= nor dirs= nor tags="})
		}
		prefix := section + ".error=" + v.Error.Str
		validateFileList(prefix+".files", perr, v.Files)
		validateFileList(prefix+".dirs", perr, v.Dirs)
		validateOverlaps(prefix+".", pwarn, v.Files, v.Dirs)
	}
}

func validateOverlaps(listname string, perr *error, files, dirs []string) {
	// First, check that dirs do not overlap.
	for i := range dirs {
		for j := range dirs {
			if i == j {
				continue
			}
			if strings.HasPrefix(dirs[i], dirs[j]+"/") {
				multierr.AppendInto(perr, &errOverlap{listname + "dirs", dirs[i], dirs[j]})
			}
		}
	}
	// Now, check that files do not overlap with any dirs.
	for i := range files {
		for j := range dirs {
			if strings.HasPrefix(files[i], dirs[j]+"/") {
				multierr.AppendInto(perr, &errOverlap{listname + "files", files[i], dirs[j]})
			}
		}
	}
}

func (c *ConfigFile) Add(add *ConfigFile) error {
	var err error

	c.FilterFiles = appendUniq("filter_files", &err, c.FilterFiles, add.FilterFiles)
	c.FilterDirs = appendUniq("filter_dirs", &err, c.FilterDirs, add.FilterDirs)
	c.FilterImages = appendUniq("filter_images", &err, c.FilterImages, add.FilterImages)
	c.CertifiedDistributions = appendUniq("certified_distributions", &err, c.CertifiedDistributions, add.CertifiedDistributions)

	c.PayloadIgnores = mergeLists("payload", &err, c.PayloadIgnores, add.PayloadIgnores)
	c.TagIgnores = mergeLists("tag", &err, c.TagIgnores, add.TagIgnores)
	c.RPMIgnores = mergeLists("rpm", &err, c.RPMIgnores, add.RPMIgnores)

	c.ErrIgnores = mergeErrIgnoreLists("[[ignore]]", &err, c.ErrIgnores, add.ErrIgnores)

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
			keyname := "[" + name + "." + k
			l.FilterFiles = appendUniq(keyname+"].filter_files", perr, l.FilterFiles, v.FilterFiles)
			l.FilterDirs = appendUniq(keyname+"].filter_dirs", perr, l.FilterDirs, v.FilterDirs)
			l.ErrIgnores = mergeErrIgnoreLists("["+keyname+".ignore]]", perr, l.ErrIgnores, v.ErrIgnores)
			main[k] = l
		} else {
			main[k] = v
		}
	}
	return main
}

func mergeErrIgnoreLists(name string, perr *error, main, add ErrIgnoreList) ErrIgnoreList {
	if len(main) == 0 {
		return add
	}

	for _, a := range add {
		// See if the error is already in the list.
		var found *ErrIgnore
		for i := range main {
			if main[i].Error.Str == a.Error.Str {
				found = &main[i]
				break
			}
		}
		if found == nil {
			// Error not found, so append to the end of the list.
			main = append(main, a)
			continue
		}
		// Item with the error found, add new files/dirs to the existing entry.
		prefix := name + ".error=" + a.Error.Str
		found.Files = appendUniq(prefix+".files", perr, found.Files, a.Files)
		found.Dirs = appendUniq(prefix+".dirs", perr, found.Dirs, a.Dirs)
	}

	return main
}
