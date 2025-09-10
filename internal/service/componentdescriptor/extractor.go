package componentdescriptor

import (
	"errors"
	"fmt"
	"io"

	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"
)

// FileExtractor is responsible for extracting a specific file from a container image layer.
type FileExtractor struct {
	// reads all data from the provided reader
	readAll func(reader io.Reader) ([]byte, error)

	tarExtractor tarExtractor
}

func NewFileExtractor(tExt tarExtractor) *FileExtractor {
	return &FileExtractor{
		readAll:      io.ReadAll,
		tarExtractor: tExt,
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
	defer layerReader.Close()

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

type tarExtractor interface {
	UnTar(tarInput []byte, name string) ([]byte, error)
}
