package internal

import (
	"os"
	"os/exec"
	"path/filepath"
)

const (
	kyma            = "kyma"
	kymaDownloadURL = "https://storage.googleapis.com/kyma-cli-unstable/kyma-darwin"
)

var DefaultKymaPath = filepath.Join(testFolder, kyma)

func SetupKymaCLI() error {
	if _, err := os.Stat(testFolder); os.IsNotExist(err) {
		if err := os.MkdirAll(testFolder, perm); err != nil {
			return err
		}
	}
	if _, err := os.Stat(DefaultKymaPath); os.IsNotExist(err) {
		if err := download(DefaultKymaPath, kymaDownloadURL); err != nil {
			return err
		}
		if err := os.Chmod(DefaultKymaPath, perm); err != nil {
			return err
		}
	}
	return nil
}

func KymaCLI(arg ...string) *exec.Cmd {
	cmd := exec.Command(DefaultKymaPath, append([]string{"--ci"}, arg...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}
