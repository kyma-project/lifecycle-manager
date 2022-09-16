package internal

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const (
	DefaultKustomizeVersion = "4.5.7"
	versionEnv              = "KUSTOMIZE_VERSION"
	kustomize               = "kustomize"
	kustomizeinstaller      = "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
)

var kustomizeBin = filepath.Join(testFolder, kustomize)

func SetupKustomize() error {
	p := testFolder

	if _, err := os.Stat(kustomizeBin); os.IsNotExist(err) {
		if runtime.GOOS == "windows" {
			if _, err := exec.LookPath("bash"); err != nil {
				return errors.New("\nBash is not installed. To install bash on windows please see http://win-bash.sourceforge.net")
			}
		}

		v := os.Getenv(versionEnv)
		if v == "" {
			v = DefaultKustomizeVersion
		}

		downloadCmd := exec.Command("curl", "-s", kustomizeinstaller)
		installCmd := exec.Command("bash", "-s", "--", v, p)

		// pipe the downloaded script to the install command
		_, err := Pipe(downloadCmd, installCmd)
		if err != nil {
			return fmt.Errorf("error installing kustomize %w", err)
		}
	}
	return nil
}

// BuildWithKustomize generates a manifest given a path using kustomize
func BuildWithKustomize(path string) ([]byte, error) {
	cmd := exec.Command(kustomizeBin, "build", path)
	return cmd.CombinedOutput()
}

// Set edits the kustomize file in the given path by setting the given key to the given value in the given resource.
// Resource can be one of: annotation, buildmetadata, image, label, nameprefix, namespace, namesuffix, replicas.
func Set(path, resource, key, value string) error {
	cmd := exec.Command(kustomizeBin, "edit", "set", key)
	cmd.Path = path
	return nil
}

// Pipe runs the src command and pipes its output to the dst commands' input.
// The output and stderr of dst are returned.
func Pipe(src, dst *exec.Cmd) (string, error) {
	var err error

	if dst.Stdin, err = src.StdoutPipe(); err != nil {
		return "", fmt.Errorf("could not pipe %s to %s: %w", src.Path, dst.Path, err)
	}

	if err := src.Start(); err != nil {
		return "", fmt.Errorf("error running %s: %w", src.Path, err)
	}

	out, err := dst.CombinedOutput()

	if waitErr := src.Wait(); waitErr != nil {
		return "", fmt.Errorf("%s: %w", err.Error(), waitErr)
	}
	return string(out), err
}
