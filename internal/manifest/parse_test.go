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

func TestPathExtractor_untarLayer(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "untar-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	var buf bytes.Buffer
	tarWriter := tar.NewWriter(&buf)

	content := []byte("file-content")
	header := &tar.Header{
		Name: "raw-manifest.yaml",
		Mode: 0o600,
		Size: int64(len(content)),
	}

	err = tarWriter.WriteHeader(header)
	require.NoError(t, err)

	_, err = tarWriter.Write(content)
	require.NoError(t, err)

	tarWriter.Close()

	tarFilePath := filepath.Join(tempDir, "test.tar")
	err = os.WriteFile(tarFilePath, buf.Bytes(), 0o600)
	require.NoError(t, err)

	extractor := manifest.NewPathExtractor(nil)

	var wg sync.WaitGroup
	numGoroutines := 5
	results := make([]string, numGoroutines)
	modTimes := make([]time.Time, numGoroutines)
	var resultErr error

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result, err := extractor.ExtractLayer(tarFilePath)
			if err != nil {
				resultErr = err
			}
			results[i] = result

			info, err := os.Stat(result)
			if err != nil {
				resultErr = err
			}
			modTimes[i] = info.ModTime()
		}(i)
	}
	wg.Wait()

	require.NoError(t, resultErr)
	assert.NotEmpty(t, results)
	assert.NotEmpty(t, modTimes)

	info, err := os.Stat(results[0])
	require.NoError(t, resultErr)
	for i := 1; i < numGoroutines; i++ {
		assert.Equal(t, info.Name(), results[i])
		assert.Equal(t, info.ModTime(), modTimes[i])
	}

	fileContent, err := os.ReadFile(results[0])
	require.NoError(t, err)
	assert.Equal(t, content, fileContent)
}
