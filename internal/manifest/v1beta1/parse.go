package v1beta1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"regexp"
	"sync"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
)

const manifestFileName = "raw-manifest.yaml"

var fileMutexMap = sync.Map{}

func GetPathFromRawManifest(ctx context.Context,
	imageSpec v1beta2.ImageSpec,
	keyChain authn.Keychain,
) (string, error) {
	imageRef := fmt.Sprintf("%s/%s@%s", imageSpec.Repo, imageSpec.Name, imageSpec.Ref)

	// check existing file
	// if file exists return existing file path
	installPath := GetFsChartPath(imageSpec)
	manifestPath := path.Join(installPath, manifestFileName)

	//We need to sync file access per installPath
	fileMutex, ok := fileMutexMap.Load(installPath)
	if !ok {
		fileMutex, ok := fileMutexMap.LoadOrStore(installPath, sync.Mutex{})
	}

	fileMutex.Lock()
	defer fileMutex.Unlock()

	dir, err := os.Open(manifestPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("opening dir for installs caused an error %s: %w", imageRef, err)
	}
	if dir != nil {
		return manifestPath, nil
	}

	// pull image layer
	layer, err := pullLayer(ctx, imageRef, keyChain)
	if err != nil {
		return "", err
	}

	// copy uncompressed manifest to install path
	blobReadCloser, err := layer.Uncompressed()
	if err != nil {
		return "", fmt.Errorf("failed fetching blob for layer %s: %w", imageRef, err)
	}
	defer blobReadCloser.Close()

	// create dir for uncompressed manifest
	if err := os.MkdirAll(installPath, fs.ModePerm); err != nil {
		return "", fmt.Errorf(
			"failure while creating installPath directory for layer %s: %w",
			imageRef, err,
		)
	}
	outFile, err := os.Create(manifestPath)
	if err != nil {
		return "", fmt.Errorf("file create failed for layer %s: %w", imageRef, err)
	}
	if _, err := io.Copy(outFile, blobReadCloser); err != nil {
		return "", fmt.Errorf("file copy storage failed for layer %s: %w", imageRef, err)
	}
	return manifestPath, io.Closer(outFile).Close()
}

func pullLayer(ctx context.Context, imageRef string, keyChain authn.Keychain) (v1.Layer, error) {
	noSchemeImageRef := ocmextensions.NoSchemeURL(imageRef)
	isInsecureLayer, _ := regexp.MatchString("^http://", imageRef)
	if isInsecureLayer {
		return crane.PullLayer(noSchemeImageRef, crane.Insecure, crane.WithAuthFromKeychain(keyChain))
	}
	return crane.PullLayer(noSchemeImageRef, crane.WithAuthFromKeychain(keyChain), crane.WithContext(ctx))
}
