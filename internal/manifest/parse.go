package manifest

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/filemutex"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"
)

var (
	ErrImageLayerPull       = errors.New("failed to pull layer")
	ErrInvalidImageSpecType = errors.New(fmt.Sprintf("invalid image spec type provided,"+
		" only '%s' '%s' are allowed", v1beta2.OciRefType, v1beta2.OciDirType))
	ErrTaintedArchive          = errors.New("content filepath tainted")
	ErrInvalidArchiveStructure = errors.New("tar archive has invalid structure, expected a single file")
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

func (p PathExtractor) FetchLayerToFile(ctx context.Context,
	imageSpec v1beta2.ImageSpec,
	keyChain authn.Keychain,
	layerName string,
) (string, error) {
	switch imageSpec.Type {
	case v1beta2.OciRefType:
		return p.getPathForFetchedLayer(ctx, imageSpec, keyChain, layerName+".yaml")
	case v1beta2.OciDirType:
		tarFile, err := p.getPathForFetchedLayer(ctx, imageSpec, keyChain, layerName+".tar")
		if err != nil {
			return "", err
		}
		extractedFile, err := p.untarLayer(tarFile)
		if err != nil {
			return "", err
		}
		return extractedFile, nil
	default:
		return "", ErrInvalidImageSpecType
	}
}

func (p PathExtractor) getPathForFetchedLayer(ctx context.Context,
	imageSpec v1beta2.ImageSpec,
	keyChain authn.Keychain,
	filename string,
) (string, error) {
	imageRef := fmt.Sprintf("%s/%s@%s", imageSpec.Repo, imageSpec.Name, imageSpec.Ref)

	installPath := getFsChartPath(imageSpec)
	manifestPath := path.Join(installPath, filename)

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

func (p PathExtractor) untarLayer(tarPath string) (string, error) {
	fileMutex, err := p.fileMutexCache.GetLocker(tarPath)
	if err != nil {
		return "", fmt.Errorf("failed to load locker from cache: %w", err)
	}
	fileMutex.Lock()
	defer fileMutex.Unlock()

	tarFile, err := os.Open(tarPath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer tarFile.Close()

	tarReader := tar.NewReader(tarFile)

	extractedFilePath := ""
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read tar: %w", err)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			return "", ErrInvalidArchiveStructure
		case tar.TypeReg:
			if strings.HasPrefix(header.Name, "._") {
				continue
			}

			extractedFilePath, err = sanitizeArchivePath(filepath.Dir(tarPath), header.Name)
			if err != nil {
				return "", fmt.Errorf("failed to sanitize archive path: %w", err)
			}

			if _, err := os.Stat(extractedFilePath); err == nil {
				return extractedFilePath, nil
			}

			outFile, err := os.Create(extractedFilePath)
			if err != nil {
				return "", fmt.Errorf("failed to create extracted file: %w", err)
			}

			var maxBytes int64 = 1024 * 1024
			if _, err := io.CopyN(outFile, tarReader, maxBytes); err != nil && !errors.Is(err, io.EOF) {
				outFile.Close()
				return "", fmt.Errorf("failed to extract from tar: %w", err)
			}
			outFile.Close()
		}

		return extractedFilePath, nil
	}

	return "", ErrInvalidArchiveStructure
}

func pullLayer(ctx context.Context, imageRef string, keyChain authn.Keychain) (containerregistryv1.Layer, error) {
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

// sanitizeArchivePath ensures the path is within the intended directory to prevent path traversal attacks (gosec:G305)
func sanitizeArchivePath(dir, path string) (string, error) {
	joinedPath := filepath.Join(dir, path)
	if strings.HasPrefix(joinedPath, filepath.Clean(dir)) {
		return joinedPath, nil
	}

	return "", fmt.Errorf("%w: %s", ErrTaintedArchive, path)
}
