package manifest_test

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/manifest"
)

func TestPathExtractor_ExtractLayer(t *testing.T) {
	content, tarFilePath := generateDummyTarFile(t)
	pathExtractor := manifest.NewPathExtractor()
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

func extractLayer(wg *sync.WaitGroup, extractor *manifest.PathExtractor, tarFilePath string, errCh chan error,
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
