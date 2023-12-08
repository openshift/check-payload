package validations

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path"

	v1 "github.com/openshift/api/image/v1"
	"github.com/openshift/check-payload/internal/rpm"
	"github.com/openshift/check-payload/internal/types"
	"k8s.io/klog/v2"
)

func ValidatePackages(ctx context.Context, cfg *types.Config, tag *v1.TagReference, mountPath string) *types.ScanResults {
	results := types.NewScanResults()

	for _, pkg := range cfg.ConfigFile.ExpectedPackages {
		klog.V(4).Infof("Scanning %v for package %v", tag.Name, pkg.RPM)

		result := types.NewScanResult().SetTag(tag)
		_, err := rpm.GetFilesFromRPM(ctx, mountPath, pkg.RPM)
		if err != nil {
			if pkg.IsRequired {
				result.SetError(err)
			} else {
				result.SetError(err).Error.SetWarning()
			}
		}
		results.Append(result)

		// Files Scan
		for _, file := range pkg.Files {
			result := types.NewScanResult().SetTag(tag).SetRPM(pkg.RPM)
			klog.V(4).Infof("Scanning %v for file %v", tag.Name, file)
			filepath := path.Join(mountPath, file.Path)
			hash, err := getFileHash(filepath)
			if err != nil {
				if err != nil {
					if pkg.IsRequired {
						result.SetError(err)
					} else {
						result.SetError(err).Error.SetWarning()
					}
				}
				results.Append(result)
				continue
			}

			if hash != file.SHA256 {
				err = fmt.Errorf("sha256 mismatch for file %v (got=%v, expected=%v)", file.Path, hash, file.SHA256)
				if pkg.IsRequired {
					result.SetError(err)
				} else {
					result.SetError(err).Error.SetWarning()
				}
				results.Append(result)
				continue
			}

			results.Append(result)
		}
	}

	return results
}

func getFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
