package manifest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/filemutex"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"
)

var ErrImageLayerPull = errors.New("failed to pull layer")

const (
	defaultFileTTLhoursMin   = 12
	defaultFileTTLhoursMax   = 24 * 7
	defaultTTLMarkerFileName = ".ttl-marker"
)

type PathExtractor struct {
	ttlMarkerFile  TTLMarkerFile
	fileMutexCache *filemutex.MutexCache
}

func NewPathExtractor(cache *filemutex.MutexCache) *PathExtractor {
	if cache == nil {
		return &PathExtractor{fileMutexCache: filemutex.NewMutexCache(nil)}
	}
	return &PathExtractor{
		fileMutexCache: cache,
		ttlMarkerFile: TTLMarkerFile{
			markerFilename:  defaultTTLMarkerFileName,
			fileTTLhoursMin: defaultFileTTLhoursMin,
			fileTTLhoursMax: defaultFileTTLhoursMax,
		},
	}
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
		isValid, err := p.ttlMarkerFile.isValid(installPath)
		if err != nil {
			return "", err
		}
		if isValid {
			return manifestPath, nil
		}

		// cache is no longer valid, remove it
		err = os.RemoveAll(installPath)
		if err != nil {
			return "", fmt.Errorf("failed to remove cache directory for image %s: %w", imageRef, err)
		}

	}

	// create image cache along with TTL
	err = p.ttlMarkerFile.create(installPath, serializeTTL(p.ttlMarkerFile.randomTTL()))
	if err != nil {
		return "", fmt.Errorf("failed to create marker file for image %s: %w", imageRef, err)
	}

	err = p.createManifestDataCache(ctx, installPath, manifestPath, imageRef, keyChain)
	if err != nil {
		p.ttlMarkerFile.remove(installPath)
		return "", fmt.Errorf("failed to create manifest data cache for image %s: %w", imageRef, err)
	}

	return manifestPath, nil
}

func (p PathExtractor) createManifestDataCache(ctx context.Context,
	installPath, manifestPath, imageRef string,
	keyChain authn.Keychain) error {
	// pull image layer
	layer, err := pullLayer(ctx, imageRef, keyChain)
	if err != nil {
		return err
	}

	// copy uncompressed manifest to install path
	blobReadCloser, err := layer.Uncompressed()
	if err != nil {
		return fmt.Errorf("failed fetching blob for layer %s: %w", imageRef, err)
	}
	defer blobReadCloser.Close()

	// create dir for uncompressed manifest
	if err := os.MkdirAll(installPath, fs.ModePerm); err != nil {
		return fmt.Errorf(
			"failure while creating installPath directory for layer %s: %w",
			imageRef, err,
		)
	}

	outFile, err := os.Create(manifestPath)
	if err != nil {
		return fmt.Errorf("file create failed for layer %s: %w", imageRef, err)
	}
	if _, err := io.Copy(outFile, blobReadCloser); err != nil {
		return fmt.Errorf("file copy storage failed for layer %s: %w", imageRef, err)
	}
	err = io.Closer(outFile).Close()
	if err != nil {
		return fmt.Errorf("failed to close io: %w", err)
	}
	return nil
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

type TTLMarkerFile struct {
	markerFilename  string
	fileTTLhoursMin int
	fileTTLhoursMax int
}

func (p TTLMarkerFile) create(dir string, value string) error {
	path := filepath.Join(dir, p.markerFilename)

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create marker file: %w", err)
	}

	_, err = file.WriteString(value)
	if err != nil {
		cerr := file.Close()
		if cerr != nil {
			return fmt.Errorf("failed to write to marker file and close it: %w", errors.Join(err, cerr))
		}
		return fmt.Errorf("failed to write to marker file: %w", err)
	}

	return file.Close()
}

func (p TTLMarkerFile) remove(dir string) error {
	path := filepath.Join(dir, p.markerFilename)

	err := os.Remove(path)
	if err != nil {
		return fmt.Errorf("failed to remove marker file: %w", err)
	}

	return nil
}

func (p TTLMarkerFile) exists(dir string) (bool, error) {
	path := filepath.Join(dir, p.markerFilename)

	fInfo, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check marker file: %w", err)
	}

	if !fInfo.Mode().IsRegular() {
		return false, fmt.Errorf("marker file is not a regular file: %w", err)
	}

	return true, nil
}

func (p TTLMarkerFile) read(dir string) (string, error) {
	path := filepath.Join(dir, p.markerFilename)

	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open marker file: %w", err)
	}

	value, err := io.ReadAll(file)

	if err != nil {
		cerr := file.Close()
		if cerr != nil {
			return "", fmt.Errorf("failed to read marker file and close it: %w", errors.Join(err, cerr))
		}
		return "", fmt.Errorf("failed to read marker file: %w", err)
	}

	err = file.Close()
	if err != nil {
		return "", fmt.Errorf("failed to close marker file: %w", err)
	}

	return string(value), nil
}

func (p TTLMarkerFile) isValid(dir string) (bool, error) {
	value, err := p.read(dir)
	if err != nil {
		return false, err
	}
	ttl, err := parseTTL(value)
	if err != nil {
		return false, fmt.Errorf("failed to parse TTL value: %s: %w", value, err)
	}

	if ttl.Before(time.Now()) {
		return false, nil
	}

	return true, nil
}

// randomTTL returns a random TTL between Now() + fileTTLhoursMin[h] and Now()+fileTTLhoursMax[h]
func (p TTLMarkerFile) randomTTL() time.Time {
	randTTL := time.Duration(rand.Intn(p.fileTTLhoursMax-p.fileTTLhoursMin)+p.fileTTLhoursMin) * time.Hour
	return time.Now().Add(randTTL)
}

func serializeTTL(ttl time.Time) string {
	return ttl.Format(time.RFC3339)
}

func parseTTL(value string) (time.Time, error) {
	return time.Parse(time.RFC3339, value)
}
