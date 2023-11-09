package integration

import (
	"path"
	"runtime"
)

const (
	prjRoot = "../.."
)

// GetProjectRoot returns the absolute path to the project's root.
func GetProjectRoot() string {
	_, filename, _, _ := runtime.Caller(0) //nolint:dogsled // Reading only the filename from the caller.

	return path.Join(path.Dir(filename), prjRoot)
}
