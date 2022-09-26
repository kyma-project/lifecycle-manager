package internal

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	DefaultKustomizeVersion = "v4.5.7"
	kustomize               = "kustomize"
)

var kustomizeBin = filepath.Join(testFolder, kustomize)
var kustomizeTar = filepath.Join(testFolder, kustomize+".tar.gz")

func SetupKustomize() error {
	kustomizeDownloadURL := fmt.Sprintf(
		"https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/%s/kustomize_%s_%s.tar.gz",
		DefaultKustomizeVersion, DefaultKustomizeVersion, kustomizeOSTarget)

	if _, err := os.Stat(testFolder); os.IsNotExist(err) {
		if err := os.MkdirAll(testFolder, perm); err != nil {
			return err
		}
	}

	if _, err := os.Stat(kustomizeTar); os.IsNotExist(err) {
		if err := download(kustomizeTar, kustomizeDownloadURL); err != nil {
			return err
		}

		kustTarZipped, err := os.Open(kustomizeTar)
		defer kustTarZipped.Close()
		if err != nil {
			return err
		}

		gzr, err := gzip.NewReader(kustTarZipped)
		if err != nil {
			return err
		}
		defer gzr.Close()
		kustTar := tar.NewReader(gzr)

		for {
			cur, err := kustTar.Next()
			if err == io.EOF {
				break
			} else if err != nil {
				return err
			}
			if cur.Typeflag != tar.TypeReg {
				continue
			}
			data, err := io.ReadAll(kustTar)
			if err != nil {
				return err
			}
			if err := os.WriteFile(kustomizeBin, data, perm); err != nil {
				return err
			}
		}

		if err := os.Remove(kustTarZipped.Name()); err != nil {
			return err
		}
	}

	return nil
}

// BuildWithKustomize generates a manifest given a path using kustomize
func BuildWithKustomize(path string) ([]byte, error) {
	cmd := exec.Command(kustomizeBin, "build", path)
	return cmd.CombinedOutput()
}
