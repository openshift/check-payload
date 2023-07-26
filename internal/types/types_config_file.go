package types

func (c *ConfigFile) Add(add *ConfigFile) {
	c.FilterFiles = append(c.FilterFiles, add.FilterFiles...)
	c.FilterDirs = append(c.FilterDirs, add.FilterDirs...)
	c.FilterImages = append(c.FilterImages, add.FilterImages...)

	c.PayloadIgnores = mergeLists(c.PayloadIgnores, add.PayloadIgnores)
	c.TagIgnores = mergeLists(c.TagIgnores, add.TagIgnores)
	c.RPMIgnores = mergeLists(c.RPMIgnores, add.RPMIgnores)

	c.ErrIgnores = append(c.ErrIgnores, add.ErrIgnores...)
}

func mergeLists(main, add map[string]IgnoreLists) map[string]IgnoreLists {
	if main == nil {
		return add
	}

	for k, v := range add {
		if l, ok := main[k]; ok {
			l.FilterFiles = append(l.FilterFiles, v.FilterFiles...)
			l.FilterDirs = append(l.FilterDirs, v.FilterDirs...)
			l.ErrIgnores = append(l.ErrIgnores, v.ErrIgnores...)
			main[k] = l
		} else {
			main[k] = v
		}
	}
	return main
}
