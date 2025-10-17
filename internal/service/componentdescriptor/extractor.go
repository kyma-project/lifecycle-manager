package componentdescriptor

import (
	"errors"
	"fmt"
	"io"

	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"
)

// fileExtractor is responsible for extracting a specific file from a container image layer.
type fileExtractor struct {
	// reads all data from the provided reader
	readAll func(reader io.Reader) ([]byte, error)

	// extracts the file with expectedName from the given tarArchive and returns its content
	unTar func(tarInput []byte, expectedName string) ([]byte, error)
}

func (fExt *fileExtractor) extractFileFromLayer(layer containerregistryv1.Layer, fileName string) ([]byte, error) {
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

	compdescBytes, err := fExt.unTar(layerBytes, fileName)
	if err != nil {
		return nil, wrap(err)
	}

	return compdescBytes, nil
}

func defaultFileExtractor() *fileExtractor {
	unTar := func(tarInput []byte, expectedName string) ([]byte, error) {
		return defaultTarExtractor(tarInput).unTar(expectedName)
	}

	return &fileExtractor{
		readAll: io.ReadAll,
		unTar:   unTar,
	}
}
