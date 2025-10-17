package componentdescriptor

import (
	"bytes"
	"errors"
	"io"
	"testing"

	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractFile(t *testing.T) {
	t.Run("should return valid error when layer content is empty", func(t *testing.T) {
		fileName := "someReallyEmptyfile"
		res, err := defaultFileExtractor().extractFileFromLayer(&mockLayer{}, fileName)
		require.Nil(t, res)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to extract data of file=\""+fileName+"\" from TAR archive")
		assert.Contains(t, err.Error(), "layer content is empty")
	})

	t.Run("should preserve original error when calling layer.Uncompressed", func(t *testing.T) {
		expectedErr := errors.New("error from Uncompressed")
		mockLayer := &mockLayer{
			errOnUncompressed: expectedErr,
		}
		res, err := defaultFileExtractor().extractFileFromLayer(mockLayer, "somefile")
		require.Nil(t, res)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("should preserve original error when calling layer.Digest", func(t *testing.T) {
		expectedErr := errors.New("error from Digest")
		mockLayer := &mockLayer{
			errOnDigest: expectedErr,
		}
		res, err := defaultFileExtractor().extractFileFromLayer(mockLayer, "somefile")
		require.Nil(t, res)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "from TAR archive in a layer with digest")
		assert.Contains(t, err.Error(), "somefile")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("should preserve original error when calling ReadAll", func(t *testing.T) {
		expectedErr := errors.New("error from ReadAll")
		subject := defaultFileExtractor()
		subject.readAll = func(r io.Reader) ([]byte, error) {
			return nil, expectedErr
		}
		res, err := subject.extractFileFromLayer(&mockLayer{}, "somefile")
		require.Nil(t, res)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
	})
}

type mockLayer struct {
	containerregistryv1.Layer

	errOnUncompressed error
	errOnDigest       error
}

func (m *mockLayer) Uncompressed() (io.ReadCloser, error) {
	if m.errOnUncompressed != nil {
		return nil, m.errOnUncompressed
	}
	emptyReader := bytes.NewReader([]byte{})
	// return a non-nil ReadCloser with empty content
	return io.NopCloser(emptyReader), nil
}

func (m *mockLayer) Digest() (containerregistryv1.Hash, error) {
	if m.errOnDigest != nil {
		return containerregistryv1.Hash{}, m.errOnDigest
	}
	return containerregistryv1.Hash{Algorithm: "foo", Hex: "bar"}, nil
}
