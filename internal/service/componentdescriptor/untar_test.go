package componentdescriptor_test

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/service/componentdescriptor"
)

func TestUnTar(t *testing.T) {
	smallInput := generateData(5 * 1024)

	t.Run("should return data from tar for small file", func(t *testing.T) {
		tarred := asTar(smallInput, "testfile1")
		res, err := defaultTarExtractor().UnTar(tarred, "testfile1")
		require.NoError(t, err)
		assert.Equal(t, smallInput, res)
	})

	t.Run("should return data from tar for large file", func(t *testing.T) {
		input := generateData(50 * 1024)
		tarred := asTar(input, "testfile2")
		res, err := defaultTarExtractor().UnTar(tarred, "testfile2")
		require.NoError(t, err)
		assert.Equal(t, input, res)
	})

	t.Run("should return error when file not found", func(t *testing.T) {
		input := generateData(9 * 1024)
		tarred := asTar(input, "testfile3")
		_, err := defaultTarExtractor().UnTar(tarred, "nonexisting")
		require.ErrorIs(t, err, componentdescriptor.ErrNotFoundInTar)
	})

	t.Run("should return error when file too large", func(t *testing.T) {
		input := generateData(150 * 1024)
		tarred := asTar(input, "testfile4")
		_, err := defaultTarExtractor().UnTar(tarred, "testfile4")
		require.ErrorIs(t, err, componentdescriptor.ErrTarTooLarge)
	})

	t.Run("should return error when input is empty", func(t *testing.T) {
		_, err := defaultTarExtractor().UnTar([]byte{}, "testfile")
		require.ErrorIs(t, err, componentdescriptor.ErrNotFoundInTar)
	})

	t.Run("should return error when input is nil", func(t *testing.T) {
		_, err := defaultTarExtractor().UnTar(nil, "testfile")
		require.ErrorIs(t, err, componentdescriptor.ErrNotFoundInTar)
	})

	t.Run("should process the data when Header size is zero", func(t *testing.T) {
		tarred := asTar([]byte{}, "testfile")
		subject := zeroSizeTarExtractor("testfile")
		res, err := subject.UnTar(tarred, "testfile")
		require.NoError(t, err)
		assert.Equal(t, []byte{}, res)
	})

	t.Run("should preserve original error when calling Next", func(t *testing.T) {
		expectedErr := errors.New("problem calling Next")
		subject := errorOnNextTarExtractor(expectedErr)
		_, err := subject.UnTar(smallInput, "testfile")
		require.Error(t, err)
		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("should process the data when Header size is zero", func(t *testing.T) {
		tarred := asTar([]byte{}, "testfile")
		subject := zeroSizeTarExtractor("testfile")
		res, err := subject.UnTar(tarred, "testfile")
		require.NoError(t, err)
		assert.Equal(t, []byte{}, res)
	})

	t.Run("should preserve original error when calling CopyN", func(t *testing.T) {
		tarred := asTar(smallInput, "testfile4")
		expectedErr := errors.New("problem calling CopyN")
		subject := errorOnCopyNTarExtractor(expectedErr)
		_, err := subject.UnTar(tarred, "testfile4")
		require.Error(t, err)
		require.ErrorIs(t, err, expectedErr)
	})
}

func generateData(size int) []byte {
	data := make([]byte, size)
	for i := range size {
		data[i] = byte(i%94 + 32) // ASCII 32 to 126
	}
	return data
}

func asTar(data []byte, filename string) []byte {
	var buf bytes.Buffer
	twriter := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name: filename,
		Mode: 0o600,
		Size: int64(len(data)),
	}
	if err := twriter.WriteHeader(hdr); err != nil {
		panic(err)
	}

	if _, err := twriter.Write(data); err != nil {
		panic(err)
	}
	if err := twriter.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func defaultTarExtractor() *componentdescriptor.TarExtractor {
	return componentdescriptor.NewTarExtractor()
}

func zeroSizeTarExtractor(name string) *componentdescriptor.TarExtractor {
	return componentdescriptor.NewTarExtractor(
		componentdescriptor.WithNextFunction(
			func(tarReader *tar.Reader) (*tar.Header, error) {
				return &tar.Header{
					Name: name,
					Size: 0,
				}, nil
			},
		),
	)
}

func errorOnNextTarExtractor(expectedErr error) *componentdescriptor.TarExtractor {
	return componentdescriptor.NewTarExtractor(
		componentdescriptor.WithNextFunction(func(tarReader *tar.Reader) (*tar.Header, error) {
			return nil, expectedErr
		}),
	)
}

func errorOnCopyNTarExtractor(expectedErr error) *componentdescriptor.TarExtractor {
	return componentdescriptor.NewTarExtractor(
		componentdescriptor.WithCopyNFunction(func(dst io.Writer, src io.Reader, n int64) (int64, error) {
			return 0, expectedErr
		}),
	)
}
