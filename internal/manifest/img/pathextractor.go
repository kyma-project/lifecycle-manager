package img

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"ocm.software/ocm/api/ocm/extensions/repositories/genericocireg/componentmapping"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/filemutex"
)

const manifestFileMode = 0o600

var (
	ErrImageLayerPull       = errors.New("failed to pull layer")
	ErrInvalidImageSpecType = fmt.Errorf("invalid image spec type provided,"+
		" only '%s' '%s' are allowed", v1beta2.OciRefType, v1beta2.OciDirType)
	ErrTaintedArchive          = errors.New("content filepath tainted")
	ErrInvalidArchiveStructure = errors.New("tar archive has invalid structure, expected a single file")
)

type layerPullerFunc func(
	ctx context.Context, imageRef string, keyChain authn.Keychain,
) (containerregistryv1.Layer, error)

type PathExtractor struct {
	fileMutexCache *filemutex.MutexCache
	puller         layerPullerFunc
}

func NewPathExtractor(puller layerPullerFunc) *PathExtractor {
	return &PathExtractor{
		fileMutexCache: filemutex.NewMutexCache(nil),
		puller:         puller,
	}
}

func (p PathExtractor) GetPathFromRawManifest(
	ctx context.Context,
	imageSpec v1beta2.ImageSpec,
	keyChain authn.Keychain,
) (string, error) {
	switch imageSpec.Type {
	case v1beta2.OciRefType:
		return p.GetPathForFetchedLayer(ctx, imageSpec, keyChain, string(v1beta2.RawManifestLayer)+".yaml")
	case v1beta2.OciDirType:
		tarFile, err := p.GetPathForFetchedLayer(ctx, imageSpec, keyChain, string(v1beta2.RawManifestLayer)+".tar")
		if err != nil {
			return "", err
		}
		extractedFile, err := p.ExtractLayer(tarFile)
		if err != nil {
			return "", err
		}
		return extractedFile, nil
	default:
		return "", ErrInvalidImageSpecType
	}
}

func (p PathExtractor) GetPathForFetchedLayer(ctx context.Context,
	imageSpec v1beta2.ImageSpec,
	keyChain authn.Keychain,
	filename string,
) (string, error) {
	imageRef := fmt.Sprintf("%s/%s/%s@%s", imageSpec.Repo, componentmapping.ComponentDescriptorNamespace,
		imageSpec.Name, imageSpec.Ref,
	)

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
		dir.Close()
		ok, verifyErr := verifyCachedManifest(manifestPath)
		if verifyErr != nil {
			return "", verifyErr
		}
		if ok {
			return manifestPath, nil
		}
		// Integrity check failed: eviction already done, fall through to re-fetch.
	}

	imgLayer, err := p.puller(ctx, imageRef, keyChain)
	if err != nil {
		return "", err
	}

	if err := writeLayerToDisk(imgLayer, installPath, manifestPath, imageRef); err != nil {
		return "", err
	}

	return manifestPath, nil
}

// writeLayerToDisk writes the uncompressed layer content to manifestPath and stores a
// SHA-256 sidecar for subsequent integrity verification.
func writeLayerToDisk(imgLayer containerregistryv1.Layer, installPath, manifestPath, imageRef string) error {
	blobReadCloser, err := imgLayer.Uncompressed()
	if err != nil {
		return fmt.Errorf("failed fetching blob for layer %s: %w", imageRef, err)
	}
	defer blobReadCloser.Close()

	if err := os.MkdirAll(installPath, fs.ModePerm); err != nil {
		return fmt.Errorf("failure while creating installPath directory for layer %s: %w", imageRef, err)
	}
	outFile, err := os.OpenFile(manifestPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, manifestFileMode)
	if err != nil {
		return fmt.Errorf("file create failed for layer %s: %w", imageRef, err)
	}

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(outFile, hasher), blobReadCloser); err != nil {
		_ = outFile.Close()
		_ = os.Remove(manifestPath)
		return fmt.Errorf("file copy storage failed for layer %s: %w", imageRef, err)
	}
	if err := io.Closer(outFile).Close(); err != nil {
		return fmt.Errorf("failed to close io: %w", err)
	}

	digest := hex.EncodeToString(hasher.Sum(nil))
	if err := os.WriteFile(sidecarDigestPath(manifestPath), []byte(digest), manifestFileMode); err != nil {
		return fmt.Errorf("failed to write digest file for layer %s: %w", imageRef, err)
	}
	return nil
}

func (p PathExtractor) ExtractLayer(tarPath string) (string, error) {
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
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read tar: %w", err)
		}

		if header.Typeflag == tar.TypeReg {
			// On macOS, tar files generated by default include a copyfile that starts with ._.
			// This condition skips those files.
			if strings.HasPrefix(header.Name, "._") {
				continue
			}
			extractedFilePath, err := sanitizeArchivePath(filepath.Dir(tarPath), header.Name)
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
			defer outFile.Close()

			if _, err := io.Copy(outFile, tarReader); err != nil { //nolint:gosec // The upstream content is
				// from managed resources, and the size is controlled, so it is safe from decompression bomb attacks.
				return "", fmt.Errorf("failed to extract from tar: %w", err)
			}
			return extractedFilePath, nil
		}
	}
	return "", ErrInvalidArchiveStructure
}

// PullLayer fetches an OCI layer from a registry.
//
// crane.PullLayer implicitly verifies the OCI layer digest (the sha256 of the compressed
// blob, i.e. imageSpec.Ref) on every fetch. The go-containerregistry library wraps the
// HTTP response body in an internal verify.ReadCloser that hashes bytes on read and returns
// an error at EOF if the digest does not match. This verification is triggered by the
// io.Copy in GetPathForFetchedLayer that fully drains the uncompressed stream.
//
// Secure vs insecure: when imageRef begins with "http://", crane.Insecure is passed and
// the layer is fetched over plain HTTP without TLS. This exposes authentication credentials
// to any in-path network observer. In production all OCI registries use HTTPS, so this
// branch is not reached there. In local test environments (e.g. k3d with a plain-HTTP
// registry), this branch is intentionally used. OCI content integrity (digest
// verification) still applies even over plain HTTP.
// Follow-up: evaluate replacing the URL-scheme detection with an explicit secure/insecure
// wiring at startup, or enabling TLS for local test registries (issue #3493).
func PullLayer(ctx context.Context, imageRef string, keyChain authn.Keychain) (containerregistryv1.Layer, error) {
	noSchemeImageRef := noSchemeURL(imageRef)
	isInsecureLayer, err := regexp.MatchString("^http://", imageRef)
	if err != nil {
		return nil, fmt.Errorf("invalid imageRef: %w", err)
	}

	if isInsecureLayer {
		imgLayer, err := crane.PullLayer(noSchemeImageRef,
			crane.Insecure, crane.WithAuthFromKeychain(keyChain), crane.WithContext(ctx))
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

// sanitizeArchivePath ensures the path is within the intended directory to prevent path traversal attacks (gosec:G305).
func sanitizeArchivePath(dir, path string) (string, error) {
	joinedPath := filepath.Join(dir, path)
	if strings.HasPrefix(joinedPath, filepath.Clean(dir)) {
		return joinedPath, nil
	}

	return "", fmt.Errorf("%w: %s", ErrTaintedArchive, path)
}

func noSchemeURL(url string) string {
	regex := regexp.MustCompile(`^https?://`)
	return regex.ReplaceAllString(url, "")
}

// sidecarDigestPath returns the path of the SHA-256 sidecar file for a cached manifest.
// The sidecar stores the hex-encoded SHA-256 of the uncompressed manifest content written
// during the initial pull, enabling integrity verification on subsequent cache hits.
func sidecarDigestPath(manifestPath string) string {
	return manifestPath + ".sha256"
}

// manifestFileDigest returns the hex-encoded SHA-256 of the file at path.
func manifestFileDigest(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for digest: %w", err)
	}
	defer file.Close()
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// verifyCachedManifest checks the on-disk manifest against its sidecar digest.
// Returns (true, nil) when the cache is valid. On any integrity failure the manifest
// and sidecar are removed and (false, nil) is returned so the caller re-fetches.
func verifyCachedManifest(manifestPath string) (bool, error) {
	sidecar := sidecarDigestPath(manifestPath)
	expected, err := os.ReadFile(sidecar)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return false, fmt.Errorf("failed to read digest file %s: %w", sidecar, err)
		}
		// Sidecar absent (legacy cache or tampered without matching sidecar update) → evict.
		if removeErr := os.Remove(manifestPath); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			return false, fmt.Errorf("failed to evict stale manifest %s: %w", manifestPath, removeErr)
		}
		return false, nil
	}

	actual, err := manifestFileDigest(manifestPath)
	if err != nil {
		return false, err
	}
	if actual != strings.TrimSpace(string(expected)) {
		_ = os.Remove(manifestPath)
		_ = os.Remove(sidecar)
		return false, nil
	}
	return true, nil
}
