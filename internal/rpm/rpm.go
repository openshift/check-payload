package rpm

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"
)

type Info struct {
	Name string // Name only
	NVRA string // Name-Version-Release.Arch
}

func GetFilesFromRPM(ctx context.Context, root, rpm string) ([]string, error) {
	klog.Infof("rpm -ql %v", rpm)
	dbpath, err := rpmDBPath(root)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "rpm", "-ql", "--dbpath", dbpath, "--root", root, rpm)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("rpm -ql error: %w (stderr=%v)", err, stderr.String())
	}

	files := []string{}
	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		files = append(files, scanner.Text())
	}
	return files, nil
}

func GetAllRPMs(ctx context.Context, root string) ([]Info, error) {
	klog.Info("rpm -qa")
	dbpath, err := rpmDBPath(root)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "rpm", "-qa", "--dbpath", dbpath, "--root", root, "--qf", "%{NAME} %{NVRA}\n")
	var stdout, stderr bytes.Buffer
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
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading rpm -qa: %w", err)
	}
	if len(rpms) == 0 {
		return nil, fmt.Errorf("no rpms found under %q", root)
	}
	return rpms, nil
}

// NameFromFile tells which rpm the given file belongs to, under a given root.
func NameFromFile(ctx context.Context, root, path string) (string, error) {
	// We can either:
	//  1: Execute host's rpm binary;
	//  2: Execute in-root rpm binary using chroot;
	//  3: Use something like https://github.com/knqyf263/go-rpmdb
	//
	// Every approach has its pros and cons.
	//  1: We have to find where rpmdb is located, as in-root configuration
	//     may differ from the host one, and we assume host rpm understands
	//     in-root rpmdb format.
	//  2: We have to trust rpm binary, and it has to be of the same arch.
	//  3: We have to trust the third-party package, and it might be slow
	//     or not have all the required functionality.
	//
	// Let's settle on 1 for now.

	dbpath, err := rpmDBPath(root)
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "rpm", "-qf", "--dbpath", dbpath, "--root", root, "--queryformat=%{NAME}", path)
	cmd.Env = append(cmd.Environ(), "LANG=C") // Do not localize error messages.
	var outbuf, errbuf bytes.Buffer
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	if err := cmd.Run(); err != nil {
		errB := errbuf.Bytes()
		// If the file does not belong to any package, do not return an error.
		if bytes.Contains(errB, []byte("is not owned by any package")) ||
			// If the file is not owned by any package, and a file
			// with the same path does not exist *on the host*, the
			// error message is ENOENT. This seems to be rpm bug, see
			// https://github.com/rpm-software-management/rpm/issues/2576.
			bytes.Contains(errB, []byte("No such file or directory")) {
			return "", nil
		}
		return "", fmt.Errorf("rpm -qf error: %w (stderr=%s)", err, strings.TrimSpace(errbuf.String()))
	}
	return strings.TrimSpace(outbuf.String()), nil
}

// rpmDBPath tries to guess the location of the rpmdb inside a given root.
// It is needed because different systems use different paths (see
// https://fedoraproject.org/wiki/Changes/RelocateRPMToUsr,
// https://coreos.github.io/rpm-ostree/#filesystem-layout).
func rpmDBPath(root string) (string, error) {
	for _, path := range []string{"/var/lib/rpm", "/usr/share/rpm", "/usr/lib/sysimage/rpm"} {
		// Do not trust symlinks as they might be absolute
		// (the alternative is to use github.com/cyphar/filepath-securejoin).
		st, err := os.Lstat(filepath.Join(root, path))
		if err == nil && st.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("can't find rpmdb under %q", root)
}
