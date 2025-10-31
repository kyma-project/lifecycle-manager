package componentdescriptor_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/service/componentdescriptor"
)

func TestExtractFileFromLayer(t *testing.T) {
	t.Run("should receive bytes from layer and return data", func(t *testing.T) {
		expectedContent := []byte("some content in the layer tar")
		tarExtractor := &mockTarExtractor{
			contentToReturn: expectedContent,
		}
		subject := componentdescriptor.NewFileExtractor(tarExtractor)

		fakeLayer := &mockLayer{
			data: []byte("some data in the layer"),
		}
		res, err := subject.ExtractFileFromLayer(fakeLayer, "somefile")
		require.NoError(t, err)
		assert.Equal(t, expectedContent, res)
		assert.Equal(t, fakeLayer.data, tarExtractor.capturedContent)
	})

	t.Run("should return valid error when layer content is empty", func(t *testing.T) {
		fileName := "someReallyEmptyfile"
		res, err := testFileExtractor().ExtractFileFromLayer(&mockLayer{}, fileName)
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
		res, err := testFileExtractor().ExtractFileFromLayer(mockLayer, "somefile")
		require.Nil(t, res)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("should preserve original error when calling layer.Digest", func(t *testing.T) {
		expectedErr := errors.New("error from Digest")
		mockLayer := &mockLayer{
			errOnDigest: expectedErr,
		}
		res, err := testFileExtractor().ExtractFileFromLayer(mockLayer, "somefile")
		require.Nil(t, res)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "from TAR archive in a layer with digest")
		assert.Contains(t, err.Error(), "somefile")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("should preserve original error when calling ReadAll", func(t *testing.T) {
		expectedErr := errors.New("error from ReadAll")
		subject := componentdescriptor.NewFileExtractor(
			componentdescriptor.NewTarExtractor(),
			componentdescriptor.WithReadAllFunction(
				func(r io.Reader) ([]byte, error) {
					return nil, expectedErr
				},
			),
		)
		res, err := subject.ExtractFileFromLayer(&mockLayer{}, "somefile")
		require.Nil(t, res)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("should preserve original error when calling UnTar", func(t *testing.T) {
		expectedErr := errors.New("error from UnTar")
		subject := componentdescriptor.NewFileExtractor(&mockTarExtractor{errorToReturn: expectedErr})
		fakeLayer := &mockLayer{
			data: []byte("some data in the layer"),
		}
		res, err := subject.ExtractFileFromLayer(fakeLayer, "somefile")
		require.Nil(t, res)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
	})
}

func testFileExtractor() *componentdescriptor.FileExtractor {
	return componentdescriptor.NewFileExtractor(
		componentdescriptor.NewTarExtractor(),
	)
}

type mockLayer struct {
	containerregistryv1.Layer

	data              []byte
	errOnUncompressed error
	errOnDigest       error
}

func (m *mockLayer) Uncompressed() (io.ReadCloser, error) {
	if m.errOnUncompressed != nil {
		return nil, m.errOnUncompressed
	}
	dataReader := bytes.NewReader(m.data)
	// return a non-nil ReadCloser
	return io.NopCloser(dataReader), nil
}

func (m *mockLayer) Digest() (containerregistryv1.Hash, error) {
	if m.errOnDigest != nil {
		return containerregistryv1.Hash{}, m.errOnDigest
	}
	return containerregistryv1.Hash{Algorithm: "foo", Hex: "bar"}, nil
}

type mockTarExtractor struct {
	capturedContent []byte
	contentToReturn []byte
	errorToReturn   error
}

func (e *mockTarExtractor) UnTar(tarInput []byte, name string) ([]byte, error) {
	if e.errorToReturn != nil {
		return nil, e.errorToReturn
	}
	e.capturedContent = tarInput
	return e.contentToReturn, nil
}
