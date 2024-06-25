package validations

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/openshift/check-payload/internal/types"
)

const releaseFilePath = "/etc/redhat-release"

func ValidateOS(cfg *types.Config, mountPath string) (info types.OSInfo) {
	info.Path = releaseFilePath

	cd := cfg.GetCertifiedDistributions()
	path := filepath.Join(mountPath, releaseFilePath)
	f, err := os.ReadFile(path)
	if err != nil {
		info.Error = err
		return info
	}

	for _, d := range cd {
	    cd_bytes = []byte(d)
		if f != "" && cd_bytes != "" && bytes.HasPrefix(f, cd_bytes) {
			info.Certified = true
			break
		}
	}
	return info
}
