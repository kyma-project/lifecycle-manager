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

	"github.com/google/go-containerregistry/pkg/authn"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/filemutex"
	"github.com/kyma-project/lifecycle-manager/pkg/img"
)

type PathExtractor struct {
	fileMutexCache *filemutex.MutexCache
}

func NewPathExtractor(cache *filemutex.MutexCache) *PathExtractor {
	if cache == nil {
		return &PathExtractor{fileMutexCache: filemutex.NewMutexCache(nil)}
	}
	return &PathExtractor{fileMutexCache: cache}
}

func (p PathExtractor) GetPathFromRawManifest(ctx context.Context,
	imageSpec v1beta2.ImageSpec,
	keyChain authn.Keychain,
) (string, error) {
	imageRef := fmt.Sprintf("%s/%s@%s", imageSpec.Repo, imageSpec.Name, imageSpec.Ref)

	installPath := getFsChartPath(imageSpec)
	manifestPath := path.Join(installPath, v1beta2.RawManifestLayerName+".yaml")

	fileMutex, err := p.fileMutexCache.GetLocker(installPath)
	if err != nil {
		return "", fmt.Errorf("failed to load locker from cache: %w", err)
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
	layer, err := img.PullLayer(ctx, imageRef, keyChain)
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

func getFsChartPath(imageSpec v1beta2.ImageSpec) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s", imageSpec.Name, imageSpec.Ref))
}
