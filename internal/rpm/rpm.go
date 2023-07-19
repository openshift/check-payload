package rpm

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"k8s.io/klog/v2"
)

type Info struct {
	Name string // Name only
	NVRA string // Name-Version-Release.Arch
}

func GetFilesFromRPM(ctx context.Context, root, rpm string) ([]string, error) {
	klog.Infof("rpm -ql %v", rpm)
	files := []string{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "rpm", "-ql", "--root", root, rpm)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return files, fmt.Errorf("rpm -ql error: %w (stderr=%v)", err, stderr.String())
	}

	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		files = append(files, scanner.Text())
	}
	return files, nil
}

func GetAllRPMs(ctx context.Context, root string) ([]Info, error) {
	klog.Info("rpm -qa")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "rpm", "-qa", "--root", root, "--qf", "%{NAME} %{NVRA}")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("rpm -qa error: %w (stderr=%v)", err, stderr.String())
	}
	rpms := []Info{}

	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		f := strings.Fields(scanner.Text())
		if len(f) != 2 {
			// Should never happen.
			continue
		}
		rpms = append(rpms, Info{Name: f[0], NVRA: f[1]})
	}
	return rpms, nil
}
