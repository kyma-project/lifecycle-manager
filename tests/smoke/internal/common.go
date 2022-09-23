package internal

import (
	"os"
	"path/filepath"
)

const (
	perm = os.ModePerm
)

var TestBins = filepath.Join("test", "bin")

var testFolder = filepath.Join(".", TestBins)
