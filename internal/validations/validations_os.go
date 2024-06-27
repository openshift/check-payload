package validations

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/openshift/check-payload/internal/types"
)

const releaseFilePath = "/etc/redhat-release"

func ValidateOS(cfg *types.Config, mountPath string) (info types.OSInfo) {
	info.Path = releaseFilePath

	cd := cfg.GetCertifiedDistributions()
	if len(cd) == 0 {
		info.Error = types.NewValidationError(types.ErrCertifiedDistributionsEmpty).SetWarning()
		return info
	}

	path := filepath.Join(mountPath, releaseFilePath)
	f, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			info.Error = types.NewValidationError(types.ErrDistributionFileMissing)
		} else {
			info.Error = types.NewValidationError(err)
		}
		return info
	}
	if len(f) == 0 {
		info.Error = types.NewValidationError(fmt.Errorf("%v is an empty file", releaseFilePath))
		return info
	}

	for _, d := range cd {
		if bytes.HasPrefix(f, []byte(d)) {
			info.Certified = true
			break
		}
	}
	return info
}
