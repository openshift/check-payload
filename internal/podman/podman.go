package podman

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/openshift/check-payload/dist/releases"
	"github.com/openshift/check-payload/internal/types"
	"github.com/openshift/check-payload/internal/validations"
	"k8s.io/klog/v2"
)

func Unmount(ctx context.Context, id string) error {
	_, err := runPodman(ctx, "image", "unmount", id)
	if err != nil {
		return err
	}
	return nil
}

func Mount(ctx context.Context, id string) (string, error) {
	stdout, err := runPodman(ctx, "image", "mount", id)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func Pull(ctx context.Context, image string, insecure bool) error {
	args := []string{"pull"}
	if insecure {
		args = append(args, "--tls-verify=false")
	}
	args = append(args, image)

	_, err := runPodman(ctx, args...)
	if err != nil {
		return err
	}
	return nil
}

func Inspect(ctx context.Context, image string, args ...string) (string, error) {
	cmdArgs := append([]string{"inspect", image}, args...)
	stdout, err := runPodman(ctx, cmdArgs...)
	if err != nil {
		return "", err
	}
	return stdout.String(), nil
}

func runPodman(ctx context.Context, args ...string) (bytes.Buffer, error) {
	klog.V(1).InfoS("podman "+args[0], "args", args[1:])
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	retry := true
	cmd := exec.CommandContext(ctx, "podman", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
again:
	if err := cmd.Run(); err != nil {
		// Retry once on Internal Server Error to improve resilience.
		if retry && strings.Contains(stderr.String(), "Internal Server Error") {
			klog.InfoS("got HTTP 500, will retry once", "stderr", stderr.String())
			stdout.Reset()
			stderr.Reset()
			retry = false
			time.Sleep(time.Second)
			goto again
		}

		// Exit code 8 is used to differentiate valid java scan returns from other execution errors.
		const javaExitCode = 8
		var exiterr *exec.ExitError
		if errors.As(err, &exiterr); exiterr.ExitCode() == javaExitCode {
			return stdout, errors.New(stderr.String())
		}
		return stdout, fmt.Errorf("podman error (args=%v) (stderr=%v) (error=%w)", args, stderr.String(), err)
	}
	return stdout, nil
}

func ScanJava(ctx context.Context, image string, javaDisabledAlgorithms []string) error {
	data, err := Inspect(ctx, image, "--format", "{{index  .Config.Entrypoint}}|{{index  .Config.Cmd}}|{{index  .Config.WorkingDir}}")
	if err != nil {
		return err
	}
	parts := strings.Split(data, "|")

	jInfo := &types.JavaComponent{
		Entrypoint: strings.Split(strings.Trim(parts[0], "[]"), ","),
		Cmd:        strings.Split(strings.Trim(parts[1], "[]"), ","),
		WorkingDir: strings.TrimSpace(parts[2]),
	}

	// This was done because java versions before 1.8 cannot use the java class `java.util.stream.Collectors`.
	// Also, java versions prior to 1.11 cannot execute .Java source files without compiling first.
	cmdArgs := []string{"run", "-it", "--rm", "--entrypoint", "", image, "java", "-XshowSettings:properties", "-version"}
	stdout, err := runPodman(ctx, cmdArgs...)
	if err != nil {
		return err
	}

	jClassVer := ""
	scanner := bufio.NewScanner(strings.NewReader(stdout.String()))
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "java.class.version =") {
			jClassVer = strings.TrimSpace(strings.Split(scanner.Text(), "=")[1])
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	javaFilePath, _, err := releases.GetJavaFile()
	if err != nil {
		return err
	}
	defer os.Remove(javaFilePath)
	algFilePath, algFile, err := releases.GetAlgorithmFile(javaDisabledAlgorithms)
	if err != nil {
		return err
	}
	defer os.Remove(algFilePath)

	cmdArgs = []string{"run", "--rm", "--entrypoint", "", "-v", javaFilePath + ":" + jInfo.WorkingDir + "/" + releases.JavaFips + ":z", "-v", algFilePath + ":" + jInfo.WorkingDir + "/" + algFile + ":z", image}
	javaClassVersion, err := semver.NewVersion(jClassVer)
	if err != nil {
		return err
	}

	if validations.JavaClassLessThan52.Check(javaClassVersion) {
		return errors.New("this scan tool supports java 1.8+")
	}
	if validations.JavaClassLessThan55.Check(javaClassVersion) {
		cmdArgs = append(cmdArgs, []string{"/bin/sh", "-c", "javac " + releases.JavaFips + " && java FIPS " + algFile}...)
	} else {
		cmdArgs = append(cmdArgs, []string{"java", releases.JavaFips, algFile}...)
	}
	stdout, err = runPodman(ctx, cmdArgs...)
	if err != nil {
		klog.Infoln(stdout.String())
		return err
	}

	return nil
}

func GetOpenshiftComponentFromImage(ctx context.Context, image string) (*types.OpenshiftComponent, error) {
	data, err := Inspect(ctx, image, "--format", "{{index  .Config.Labels \"com.redhat.component\" }}|{{index  .Config.Labels \"io.openshift.build.source-location\" }}|{{index .Config.Labels \"io.openshift.maintainer.component\"}}|{{index .Config.Labels \"com.redhat.delivery.operator.bundle\"}}")
	if err != nil {
		return nil, err
	}
	parts := strings.Split(data, "|")

	oc := &types.OpenshiftComponent{}
	oc.Component = strings.TrimSpace(parts[0])
	oc.SourceLocation = strings.TrimSpace(parts[1])
	oc.MaintainerComponent = strings.TrimSpace(parts[2])
	oc.IsBundle = strings.EqualFold(strings.TrimSpace(parts[3]), "true")
	return oc, nil
}
