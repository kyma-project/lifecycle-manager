package componentdescriptor

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
)

const (
	MaxDescriptorSizeBytes = 100 * 1024 // 100KiB, our average is around 4KiB
	TarReadChunkSize       = 10 * 1024  // 10KiB, for our average size we'll read it in one go
)

// tarExtractor is responsible for extracting named files from a tar archive.
type tarExtractor struct {
	next  func() (*tar.Header, error)                 // tar.Reader.Next()
	copyN func(dst io.Writer, n int64) (int64, error) // modified io.CopyN()
}

// unTar extracts the file with provided name and returns its content.
func (tarex *tarExtractor) unTar(name string) ([]byte, error) {
	for {
		hdr, err := tarex.next()
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
				maxSize = MaxDescriptorSizeBytes // sanity
			}
			if maxSize > MaxDescriptorSizeBytes { // DoS protection
				return nil, fmt.Errorf("%s %w", name, ErrTarTooLarge)
			}
			for buf.Len() < int(maxSize) { // DoS protection: read in chunks
				if _, err := tarex.copyN(&buf, TarReadChunkSize); err != nil {
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

func defaultTarExtractor(data []byte) *tarExtractor {
	reader := tar.NewReader(bytes.NewReader(data))
	copyN := func(dst io.Writer, n int64) (int64, error) {
		return io.CopyN(dst, reader, n)
	}

	return &tarExtractor{
		next:  reader.Next,
		copyN: copyN,
	}
}
