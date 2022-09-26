package internal

import (
	"os/exec"
)

// BuildWithKustomize generates a manifest given a path using kustomize
func BuildWithKustomize(path string) ([]byte, error) {
	cmd := exec.Command("kustomize", "build", path)
	return cmd.CombinedOutput()
}
