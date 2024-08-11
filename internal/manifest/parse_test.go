package manifest

import (
	"archive/tar"
	"bytes"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestPathExtractor_untarLayer(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "untar-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	var buf bytes.Buffer
	tarWriter := tar.NewWriter(&buf)

	content := []byte("file-content")
	header := &tar.Header{
		Name: "raw-manifest.yaml",
		Mode: 0600,
		Size: int64(len(content)),
	}

	err = tarWriter.WriteHeader(header)
	assert.NoError(t, err)

	_, err = tarWriter.Write(content)
	assert.NoError(t, err)

	tarWriter.Close()

	tarFilePath := filepath.Join(tempDir, "test.tar")
	err = os.WriteFile(tarFilePath, buf.Bytes(), 0600)
	assert.NoError(t, err)

	extractor := NewPathExtractor(nil)

	var wg sync.WaitGroup
	numGoroutines := 5
	results := make([]string, numGoroutines)
	modTimes := make([]time.Time, numGoroutines)
	var resultErr error

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result, err := extractor.untarLayer(tarFilePath)
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

	assert.NoError(t, resultErr)
	assert.NotEmpty(t, results)
	assert.NotEmpty(t, modTimes)

	for i := 1; i < numGoroutines; i++ {
		assert.Equal(t, results[0], results[i])
		assert.Equal(t, modTimes[0], modTimes[i])
	}

	fileContent, err := os.ReadFile(results[0])
	assert.NoError(t, err)
	assert.Equal(t, content, fileContent)
}
