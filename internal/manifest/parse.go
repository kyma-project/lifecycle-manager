package manifest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"
)

//nolint:gochecknoglobals
var (
	ErrImageLayerPull = errors.New("failed to pull layer")
	fileMutexMap      = sync.Map{}
)

func GetPathFromRawManifest(ctx context.Context,
	imageSpec v1beta2.ImageSpec,
	keyChain authn.Keychain,
) (string, error) {
	imageRef := fmt.Sprintf("%s/%s@%s", imageSpec.Repo, imageSpec.Name, imageSpec.Ref)

	// check existing file
	// if file exists return existing file path
	installPath := getFsChartPath(imageSpec)
	manifestPath := path.Join(installPath, v1beta2.RawManifestLayerName+".yaml")

	fileMutex := getLockerForPath(installPath)
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
	err = io.Closer(outFile).Close()
	if err != nil {
		return manifestPath, fmt.Errorf("failed to close io: %w", err)
	}
	return manifestPath, nil
}

func pullLayer(ctx context.Context, imageRef string, keyChain authn.Keychain) (v1.Layer, error) {
	noSchemeImageRef := ocmextensions.NoSchemeURL(imageRef)
	isInsecureLayer, err := regexp.MatchString("^http://", imageRef)
	if err != nil {
		return nil, fmt.Errorf("invalid imageRef: %w", err)
	}

	if isInsecureLayer {
		imgLayer, err := crane.PullLayer(noSchemeImageRef, crane.Insecure, crane.WithAuthFromKeychain(keyChain))
		if err != nil {
			return nil, fmt.Errorf("%s due to: %w", ErrImageLayerPull.Error(), err)
		}
		return imgLayer, nil
	}
	imgLayer, err := crane.PullLayer(noSchemeImageRef, crane.WithAuthFromKeychain(keyChain), crane.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("%s due to: %w", ErrImageLayerPull.Error(), err)
	}
	return imgLayer, nil
}

func getFsChartPath(imageSpec v1beta2.ImageSpec) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s", imageSpec.Name, imageSpec.Ref))
}

// getLockerForPath always returns the same sync.Locker instance for given path argument.
func getLockerForPath(path string) sync.Locker {
	val, ok := fileMutexMap.Load(path)
	if !ok {
		val, _ = fileMutexMap.LoadOrStore(path, &sync.Mutex{})
	}
	// no alternative here
	//nolint:forcetypeassert
	return val.(*sync.Mutex)
}
