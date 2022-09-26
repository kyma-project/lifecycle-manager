package internal

import (
	"os"
	"os/exec"
)

func KymaCLI(arg ...string) *exec.Cmd {
	cmd := exec.Command("kyma", append([]string{"--ci"}, arg...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}
