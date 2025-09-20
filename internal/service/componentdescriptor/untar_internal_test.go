package componentdescriptor

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnTar(t *testing.T) {
	t.Run("should return data from tar for small file", func(t *testing.T) {
		input := generateData(5 * 1024)
		tarred := asTar(input, "testfile1")
		res, err := unTar(mockIOHelper(tarred), "testfile1")
		require.NoError(t, err)
		assert.Equal(t, input, res)
	})

	t.Run("should return data from tar for large file", func(t *testing.T) {
		input := generateData(50 * 1024)
		tarred := asTar(input, "testfile2")
		res, err := unTar(mockIOHelper(tarred), "testfile2")
		require.NoError(t, err)
		assert.Equal(t, input, res)
	})

	t.Run("should return error when file not found", func(t *testing.T) {
		input := generateData(9 * 1024)
		tarred := asTar(input, "testfile3")
		_, err := unTar(mockIOHelper(tarred), "nonexisting")
		require.ErrorIs(t, err, ErrNotFoundInTar)
	})

	t.Run("should return error when file too large", func(t *testing.T) {
		input := generateData(150 * 1024)
		tarred := asTar(input, "testfile4")
		_, err := unTar(mockIOHelper(tarred), "testfile4")
		require.ErrorIs(t, err, ErrTarTooLarge)
	})

	t.Run("should return error when input is empty", func(t *testing.T) {
		_, err := unTar(mockIOHelper([]byte{}), "testfile")
		require.ErrorIs(t, err, ErrNotFoundInTar)
	})

	t.Run("should preserve original error when calling Next", func(t *testing.T) {
		expectedErr := errors.New("problem calling Next")
		helper := mockIOHelper(asTar(generateData(5*1024), "testfile"))
		helper.errOnNext = expectedErr
		_, err := unTar(helper, "testfile")
		require.Error(t, err)
		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("should preserve original error when calling CopyN", func(t *testing.T) {
		expectedErr := errors.New("problem calling CopyN")
		helper := mockIOHelper(asTar(generateData(5*1024), "testfile"))
		helper.errOnCopyN = expectedErr
		_, err := unTar(helper, "testfile")
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

func mockIOHelper(data []byte) *mockHelper {
	res := mockHelper{
		goodHelper: &defaultUntarIOHelper{tarReader: tar.NewReader(bytes.NewReader(data))},
	}
	return &res
}

type mockHelper struct {
	goodHelper untarIoHelper
	errOnNext  error
	errOnCopyN error
}

func (b *mockHelper) Next() (*tar.Header, error) {
	if b.errOnNext != nil {
		return nil, b.errOnNext
	}
	return b.goodHelper.Next()
}

func (b *mockHelper) CopyN(dst io.Writer, n int64) (int64, error) {
	if b.errOnCopyN != nil {
		return 0, b.errOnCopyN
	}
	return b.goodHelper.CopyN(dst, n)
}
