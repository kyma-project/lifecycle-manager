package componentdescriptor

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
)

const (
	maxDescriptorSizeBytes = 100 * 1024 // 100KiB, our average is around 4KiB
	tarReadChunkSize       = 10 * 1024  // 10KiB, for our average size we'll read it in one go
)

// TarExtractor is responsible for extracting named files from a tar archive.
type TarExtractor struct {
	next  func(tarReader *tar.Reader) (*tar.Header, error)           // wrapper over tar.Reader.Next()
	copyN func(dst io.Writer, src io.Reader, n int64) (int64, error) // wrapper over io.CopyN()
}

func NewTarExtractor() *TarExtractor {
	return &TarExtractor{
		next:  (*tar.Reader).Next,
		copyN: io.CopyN,
	}
}

// unTar extracts the file with provided name and returns its content.
func (tarex *TarExtractor) UnTar(tarInput []byte, name string) ([]byte, error) {
	tarReader := tar.NewReader(bytes.NewReader(tarInput))
	for {
		hdr, err := tarex.next(tarReader)
		if errors.Is(err, io.EOF) {
			break // end of archive
		}
		if err != nil {
			return nil, err
		}

		if hdr.Name == name {
			var buf bytes.Buffer
			maxSize := hdr.Size
			if maxSize <= 0 {
				maxSize = maxDescriptorSizeBytes // sanity
			}
			if maxSize > maxDescriptorSizeBytes { // DoS protection
				return nil, fmt.Errorf("%s %w", name, ErrTarTooLarge)
			}
			for buf.Len() < int(maxSize) { // DoS protection: read in chunks
				if _, err := tarex.copyN(&buf, tarReader, tarReadChunkSize); err != nil {
					if errors.Is(err, io.EOF) {
						break
					}
					return nil, err
				}
			}

			return buf.Bytes(), nil
		}
	}

	return nil, fmt.Errorf("%s %w", name, ErrNotFoundInTar)
}
