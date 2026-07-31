package img_test

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/img"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

func TestPathExtractor_ExtractLayer(t *testing.T) {
	content, tarFilePath := generateDummyTarFile(t)
	pathExtractor := img.NewPathExtractor()
	numGoroutines := 5
	resultCh := make(chan string, numGoroutines)
	modTimeCh := make(chan time.Time, numGoroutines)
	errCh := make(chan error)
	var waitGroup sync.WaitGroup
	for range numGoroutines {
		waitGroup.Add(1)
		go extractLayer(&waitGroup, pathExtractor, tarFilePath, errCh, resultCh, modTimeCh)
	}
	go func() {
		waitGroup.Wait()
		close(resultCh)
		close(modTimeCh)
		close(errCh)
	}()
	results := make([]string, 0)
	modTimes := make([]time.Time, 0)

	for err := range errCh {
		require.NoError(t, err)
	}
	for result := range resultCh {
		results = append(results, result)
	}
	for modTime := range modTimeCh {
		modTimes = append(modTimes, modTime)
	}

	assert.Len(t, results, numGoroutines)
	assert.Len(t, modTimes, numGoroutines)

	extractedPath := results[0]
	info, err := os.Stat(extractedPath)
	require.NoError(t, err)
	for i := range numGoroutines {
		assert.Equal(t, extractedPath, results[i])
		assert.Equal(t, info.ModTime(), modTimes[i])
	}

	fileContent, err := os.ReadFile(extractedPath)
	require.NoError(t, err)
	assert.Equal(t, content, fileContent)
}

func TestPathExtractor_FetchLayerToFile(t *testing.T) {
	const commonRepo = "europe-west3-docker.pkg.dev/sap-kyma-jellyfish-dev/template-operator/component-descriptors"

	tests := []struct {
		name     string
		fileName string
		want     img.Layer
	}{
		{
			"should fetch raw-manifest layer with oci-dir type",
			"template-operator.yaml",
			img.Layer{
				LayerName: "raw-manifest",
				LayerRepresentation: &img.OCI{
					Repo: "normally-determined-by-OCM-component-descriptor-but-" +
						"in-our-code-is-overridden-with-an-explicit-value",
					Name: testutils.DefaultComponentName,
					Ref:  "sha256:d2cc278224a71384b04963a83e784da311a268a2b3fa8732bc31e70ca0c5bc52",
					Type: "oci-dir",
				},
			},
		},
		{
			"should fetch raw-manifest layer with oci-ref type",
			"raw-manifest.yaml",
			img.Layer{
				LayerName: "raw-manifest",
				LayerRepresentation: &img.OCI{
					Repo: "normally-determined-by-OCM-component-descriptor-but-" +
						"in-our-code-is-overridden-with-an-explicit-value",
					Name: testutils.DefaultComponentName,
					Ref:  "sha256:1ea2baf45791beafabfee533031b715af8f7a4ffdfbbf30d318f52f7652c36ca",
					Type: "oci-ref",
				},
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			p := img.NewPathExtractor()
			imageSpec, err := testCase.want.ConvertToImageSpec(commonRepo)
			require.NoError(t, err)
			extractedFilePath, err := p.GetPathFromRawManifest(t.Context(), *imageSpec, authn.DefaultKeychain)
			require.NoError(t, err)
			assert.Contains(t, extractedFilePath,
				fmt.Sprintf("%s/%s", imageSpec.Ref, testCase.fileName))
		})
	}
}

// TestPathExtractor_GetPathForFetchedLayer_TamperedCacheIsEvictedAndRefetched verifies
// the defense-in-depth guarantee: a tampered cached manifest (no matching sidecar digest)
// is evicted and replaced with freshly fetched content.
func TestPathExtractor_GetPathForFetchedLayer_TamperedCacheIsEvictedAndRefetched(t *testing.T) {
	const legitimateContent = "legitimate-manifest-content"

	imageSpec := v1beta2.ImageSpec{
		Repo: "example.com",
		Name: "test-module",
		Ref:  "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Type: v1beta2.OciRefType,
	}

	// Mirror getFsChartPath + filename to pre-populate the cache directory.
	installPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s", imageSpec.Name, imageSpec.Ref))
	manifestPath := filepath.Join(installPath, string(v1beta2.RawManifestLayer)+".yaml")

	t.Cleanup(func() { os.RemoveAll(installPath) })
	require.NoError(t, os.RemoveAll(installPath))
	require.NoError(t, os.MkdirAll(installPath, 0o755))

	// Write tampered content with no sidecar digest — simulates a cache poisoning attempt.
	require.NoError(t, os.WriteFile(manifestPath, []byte("tampered-content"), 0o600))

	extractor := img.NewPathExtractor(img.WithLayerPuller(
		func(_ context.Context, _ string, _ authn.Keychain) (containerregistryv1.Layer, error) {
			return &staticLayer{content: []byte(legitimateContent)}, nil
		},
	))

	result, err := extractor.GetPathFromRawManifest(t.Context(), imageSpec, authn.DefaultKeychain)
	require.NoError(t, err)

	got, err := os.ReadFile(result)
	require.NoError(t, err)
	assert.Equal(t, []byte(legitimateContent), got, "tampered cache must be replaced with legitimate content")
}

// TestPathExtractor_GetPathForFetchedLayer_ValidCacheIsReused verifies that a cache entry
// with a matching sidecar digest is served without calling the layer puller again.
func TestPathExtractor_GetPathForFetchedLayer_ValidCacheIsReused(t *testing.T) {
	const cachedContent = "cached-manifest-content"

	imageSpec := v1beta2.ImageSpec{
		Repo: "example.com",
		Name: "test-module-cached",
		Ref:  "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Type: v1beta2.OciRefType,
	}

	installPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s", imageSpec.Name, imageSpec.Ref))
	t.Cleanup(func() { os.RemoveAll(installPath) })
	require.NoError(t, os.RemoveAll(installPath))

	pullerCallCount := 0
	extractor := img.NewPathExtractor(img.WithLayerPuller(
		func(_ context.Context, _ string, _ authn.Keychain) (containerregistryv1.Layer, error) {
			pullerCallCount++
			return &staticLayer{content: []byte(cachedContent)}, nil
		},
	))

	// First call: cache miss — puller invoked, sidecar written.
	result1, err := extractor.GetPathFromRawManifest(t.Context(), imageSpec, authn.DefaultKeychain)
	require.NoError(t, err)
	assert.Equal(t, 1, pullerCallCount)

	// Second call: cache hit with valid sidecar — puller must NOT be called again.
	result2, err := extractor.GetPathFromRawManifest(t.Context(), imageSpec, authn.DefaultKeychain)
	require.NoError(t, err)
	assert.Equal(t, result1, result2)
	assert.Equal(t, 1, pullerCallCount, "puller must not be called when cache is valid")
}

// staticLayer is a minimal v1.Layer implementation returning fixed content from Uncompressed.
type staticLayer struct {
	content []byte
}

// zeroHash is a placeholder hash returned by staticLayer for Digest and DiffID.
// The value is irrelevant for tests that only exercise the caching path.
const zeroHash = "0000000000000000000000000000000000000000000000000000000000000000"

func (s *staticLayer) Digest() (containerregistryv1.Hash, error) {
	return containerregistryv1.Hash{Algorithm: "sha256", Hex: zeroHash}, nil
}

func (s *staticLayer) DiffID() (containerregistryv1.Hash, error) {
	return containerregistryv1.Hash{Algorithm: "sha256", Hex: zeroHash}, nil
}

func (s *staticLayer) Compressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.content)), nil
}

func (s *staticLayer) Uncompressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.content)), nil
}

func (s *staticLayer) Size() (int64, error) { return int64(len(s.content)), nil }

func (s *staticLayer) MediaType() (types.MediaType, error) { return types.OCIUncompressedLayer, nil }

func generateDummyTarFile(t *testing.T) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	tarWriter := tar.NewWriter(&buf)

	content := []byte("file-content")
	header := &tar.Header{
		Name: "raw-manifest.yaml",
		Mode: 0o600,
		Size: int64(len(content)),
	}

	err := tarWriter.WriteHeader(header)
	require.NoError(t, err)

	_, err = tarWriter.Write(content)
	require.NoError(t, err)

	err = tarWriter.Close()
	require.NoError(t, err)
	tarFilePath := filepath.Join(os.TempDir(), "test.tar")
	err = os.WriteFile(tarFilePath, buf.Bytes(), 0o600)
	require.NoError(t, err)
	return content, tarFilePath
}

func extractLayer(wg *sync.WaitGroup, extractor *img.PathExtractor, tarFilePath string, errCh chan error,
	resultCh chan string, modTimeCh chan time.Time,
) {
	defer wg.Done()
	result, err := extractor.ExtractLayer(tarFilePath)
	if err != nil {
		errCh <- err
		return
	}

	info, err := os.Stat(result)
	if err != nil {
		errCh <- err
		return
	}

	resultCh <- result
	modTimeCh <- info.ModTime()
}
