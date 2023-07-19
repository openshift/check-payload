package types

import (
	"strings"

	imagev1 "github.com/openshift/api/image/v1"
	"k8s.io/klog/v2"
)

func (c *Config) Log() {
	klog.Infof("using config %+v", c)
}

// isMatch tells if path equals to one of the entries.
func isMatch(path string, entries []string) bool {
	for _, f := range entries {
		if f == path {
			return true
		}
	}
	return false
}

func (c *Config) isFileIgnoredByComponent(path string, component *OpenshiftComponent) bool {
	if component == nil {
		return false
	}
	if op, ok := c.PayloadIgnores[component.Component]; ok {
		return isMatch(path, op.FilterFiles)
	}
	return false
}

func (c *Config) isDirIgnoredByComponent(path string, component *OpenshiftComponent) bool {
	if component == nil {
		return false
	}
	if op, ok := c.PayloadIgnores[component.Component]; ok {
		return isMatch(path, op.FilterDirs)
	}
	return false
}

func (c *Config) isFileIgnoredByTag(path string, tag *imagev1.TagReference) bool {
	if tag == nil {
		return false
	}
	if op, ok := c.TagIgnores[tag.Name]; ok {
		return isMatch(path, op.FilterFiles)
	}
	return false
}

func (c *Config) isFileIgnoredByRPM(path string, rpm string) bool {
	if op, ok := c.RPMIgnores[rpm]; ok {
		return isMatch(path, op.FilterFiles)
	}
	return false
}

func (c *Config) IgnoreFile(path string) bool {
	return isMatch(path, c.FilterFiles)
}

func (c *Config) IgnoreFileWithComponent(path string, component *OpenshiftComponent) bool {
	return c.isFileIgnoredByComponent(path, component) || c.IgnoreFile(path)
}

func (c *Config) IgnoreDir(path string) bool {
	return isMatch(path, c.FilterDirs)
}

func (c *Config) IgnoreFileWithTag(path string, tag *imagev1.TagReference) bool {
	return c.isFileIgnoredByTag(path, tag)
}

func (c *Config) IgnoreFileByRpm(path string, rpm string) bool {
	return c.isFileIgnoredByRPM(path, rpm)
}

func (c *Config) IgnoreDirWithComponent(path string, component *OpenshiftComponent) bool {
	return c.isDirIgnoredByComponent(path, component) || c.IgnoreDir(path)
}

// IgnoreDirPrefix is similar to IgnoreDir. The difference is, this method
// performs a a prefix match, meaning that "/a/b/c" path supplied will
// return true if c.FilterDirs contains "/a" or "/a/b".
// This method should be used from code that receives the list of files
// (such as rpm -ql input), rather than traverses a file tree.
func (c *Config) IgnoreDirPrefix(path string) bool {
	for _, dir := range c.FilterDirs {
		if strings.HasPrefix(path, dir+"/") {
			return true
		}
	}
	return false
}
