package rpm

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"k8s.io/klog/v2"
)

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

func GetAllRPMs(ctx context.Context, root string) ([]string, error) {
	klog.Info("rpm -qa")
	rpms := []string{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "rpm", "-qa", "--root", root)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return rpms, fmt.Errorf("rpm -qa error: %w (stderr=%v)", err, stderr.String())
	}

	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		rpms = append(rpms, scanner.Text())
	}
	return rpms, nil
}
