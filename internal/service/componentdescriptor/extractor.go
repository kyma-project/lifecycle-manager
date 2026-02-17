package componentdescriptor

import (
	"errors"
	"fmt"
	"io"

	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"
)

type tarExtractor interface {
	UnTar(tarInput []byte, name string) ([]byte, error)
}

type readAllFunc func(reader io.Reader) ([]byte, error)

// FileExtractor is responsible for extracting a specific file from a container image layer.
type FileExtractor struct {
	// reads all data from the provided reader
	readAll readAllFunc

	tarExtractor tarExtractor
}

func NewFileExtractor(tExt tarExtractor,
	opts ...func(*FileExtractor) *FileExtractor,
) *FileExtractor {
	fileExtractor := &FileExtractor{
		readAll:      io.ReadAll,
		tarExtractor: tExt,
	}

	for _, opt := range opts {
		fileExtractor = opt(fileExtractor)
	}

	return fileExtractor
}

// WithReadAllFunction is a low level primitive that replaces the default io.ReadAll function.
func WithReadAllFunction(f readAllFunc) func(*FileExtractor) *FileExtractor {
	return func(fExt *FileExtractor) *FileExtractor {
		fExt.readAll = f
		return fExt
	}
}

func (fExt *FileExtractor) ExtractFileFromLayer(layer containerregistryv1.Layer, fileName string) ([]byte, error) {
	wrap := func(err error) error {
		digest, derr := layer.Digest()
		if derr != nil {
			err = errors.Join(err, derr)
		}
		return fmt.Errorf("failed to extract data of file=%q from TAR archive in a layer with digest=%q: %w",
			fileName, digest, err)
	}

	layerReader, err := layer.Uncompressed()
	if err != nil {
		return nil, wrap(err)
	}
	defer func() {
		_ = layerReader.Close()
	}()

	layerBytes, err := fExt.readAll(layerReader)
	if err != nil {
		return nil, wrap(err)
	}

	if len(layerBytes) == 0 {
		return nil, wrap(ErrLayerEmpty)
	}

	compdescBytes, err := fExt.tarExtractor.UnTar(layerBytes, fileName)
	if err != nil {
		return nil, wrap(err)
	}

	return compdescBytes, nil
}
