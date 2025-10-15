package componentdescriptor

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"
	"ocm.software/ocm/api/ocm/compdesc"
	"ocm.software/ocm/api/ocm/extensions/repositories/genericocireg"

	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
)

const (
	MaxDescriptorSizeBytes      = 100 * 1024 // 100KiB, our average is around 4KiB
	TarReadChunkSize            = 10 * 1024  // 10KiB, for our average size we'll read it in one go
	ComponentDescriptorFileName = compdesc.ComponentDescriptorFileName
)

var (
	ErrInvalidArg       = errors.New("invalid argument")
	ErrLayerNil         = errors.New("ComponentDescriptorLayer is nil in ComponentDescriptorConfig")
	ErrLayerDigestEmpty = errors.New("ComponentDescriptorLayer.Digest is empty in ComponentDescriptorConfig")
	ErrNotFoundInTar    = errors.New("not found in TAR archive")
	ErrTarTooLarge      = errors.New("entry in the TAR archive is too large")
	ErrLayerEmpty       = errors.New("layer content is empty")
	ErrDecode           = errors.New("failed to decode component descriptor")
)

type OCIRepository interface {
	GetConfigFile(ctx context.Context, name, tag string) ([]byte, error)
	PullLayer(ctx context.Context, name, tag, digest string) (containerregistryv1.Layer, error)
}

type Service struct {
	ociRepository OCIRepository
}

func NewService(ociRepository OCIRepository) (*Service, error) {
	if ociRepository == nil {
		return nil, fmt.Errorf("ociRepository must not be nil: %w", ErrInvalidArg)
	}

	return &Service{
		ociRepository: ociRepository,
	}, nil
}

func commonErrMsg(ocmi ocmidentity.Component) string {
	return fmt.Sprintf("ocm artifact with name=%q and version=%q",
		ocmi.Name(), ocmi.Version())
}

func (s *Service) GetComponentDescriptor(ctx context.Context, ocmi ocmidentity.Component) (*types.Descriptor, error) {
	// Fetch the image config to get the ComponentDescriptor layer info
	configBytes, err := s.ociRepository.GetConfigFile(ctx, ocmi.Name(), ocmi.Version())
	if err != nil {
		return nil, fmt.Errorf("failed to get config file for %s: %w", commonErrMsg(ocmi), err)
	}

	ocmArtifactConfig := genericocireg.ComponentDescriptorConfig{}
	err = json.Unmarshal(configBytes, &ocmArtifactConfig)
	if err != nil {
		return nil,
			fmt.Errorf("failed to unmarshal config data into ComponentDescriptorConfig for %s: %w",
				commonErrMsg(ocmi), err)
	}

	if ocmArtifactConfig.ComponentDescriptorLayer == nil {
		return nil, fmt.Errorf("%w for %s", ErrLayerNil, commonErrMsg(ocmi))
	}

	compDescLayerDigest := ocmArtifactConfig.ComponentDescriptorLayer.Digest
	if string(compDescLayerDigest) == "" {
		return nil,
			fmt.Errorf("%w for %s", ErrLayerDigestEmpty, commonErrMsg(ocmi))
	}

	layer, err := s.ociRepository.PullLayer(ctx, ocmi.Name(), ocmi.Version(), string(compDescLayerDigest))
	if err != nil {
		return nil, fmt.Errorf("failed to pull layer for ocm artifact with name=%q, version=%q and digest=%q: %w",
			ocmi.Name(), ocmi.Version(), string(compDescLayerDigest), err)
	}

	ioh := &defaultExtractFileIOHelper{readAllFunc: io.ReadAll}
	compdescBytes, err := extractFile(ioh, layer, ComponentDescriptorFileName)
	if err != nil {
		return nil,
			fmt.Errorf("failed to extract component descriptor from layer fetched from %s with digest=%q: %w",
				commonErrMsg(ocmi), string(compDescLayerDigest), err)
	}

	descriptor, err := deserialize(compdescBytes, ocmi)
	if err != nil {
		return nil, err
	}

	return &types.Descriptor{
		ComponentDescriptor: descriptor,
	}, nil
}

// deserialize decodes the component descriptor from its serialized form.
func deserialize(compdescBytes []byte, ocmi ocmidentity.Component) (*compdesc.ComponentDescriptor, error) {
	desc, err := compdesc.Decode(compdescBytes)
	if err != nil {
		return nil, fmt.Errorf("%w fetched from %s: %w",
			ErrDecode, commonErrMsg(ocmi), err)
	}
	return desc, nil
}

func extractFile(ioh extractFileIOHelper, layer containerregistryv1.Layer, fileName string) ([]byte, error) {
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

	layerBytes, err := ioh.ReadAll(layerReader)
	if err != nil {
		return nil, wrap(err)
	}

	if len(layerBytes) == 0 {
		return nil, wrap(ErrLayerEmpty)
	}

	tarIOh := &defaultUntarIOHelper{tarReader: tar.NewReader(bytes.NewReader(layerBytes))}
	compdescBytes, err := unTar(tarIOh, fileName)
	if err != nil {
		return nil, wrap(err)
	}

	return compdescBytes, nil
}

type extractFileIOHelper interface {
	ReadAll(r io.Reader) ([]byte, error) // Reads all data from the reader
}

type defaultExtractFileIOHelper struct {
	readAllFunc func(r io.Reader) ([]byte, error)
}

func (d *defaultExtractFileIOHelper) ReadAll(r io.Reader) ([]byte, error) {
	return d.readAllFunc(r)
}

// unTar extracts the file with expectedName from the given tarBytes and returns its content.
func unTar(ioh untarIoHelper, expectedName string) ([]byte, error) {
	for {
		hdr, err := ioh.Next()
		if errors.Is(err, io.EOF) {
			break // end of archive
		}
		if err != nil {
			return nil, err
		}

		if hdr.Name == expectedName {
			var buf bytes.Buffer
			maxSize := hdr.Size
			if maxSize <= 0 {
				maxSize = MaxDescriptorSizeBytes // sanity
			}
			if maxSize > MaxDescriptorSizeBytes { // DoS protection
				return nil, fmt.Errorf("%s %w", expectedName, ErrTarTooLarge)
			}
			for buf.Len() < int(maxSize) { // DoS protection: read in chunks
				if _, err := ioh.CopyN(&buf, TarReadChunkSize); err != nil {
					if errors.Is(err, io.EOF) {
						break
					}
					return nil, err
				}
			}

			return buf.Bytes(), nil
		}
	}

	return nil, fmt.Errorf("%s %w", expectedName, ErrNotFoundInTar)
}

// untarIoHelper abstracts the methods of tar.Reader and io package used in unTar function, for better testability.
type untarIoHelper interface {
	Next() (*tar.Header, error)                              // tar.Reader.Next()
	CopyN(dst io.Writer, n int64) (written int64, err error) // modified io.CopyN()
}

type defaultUntarIOHelper struct {
	tarReader *tar.Reader
}

func (d *defaultUntarIOHelper) Next() (*tar.Header, error) {
	return d.tarReader.Next() //nolint:wrapcheck // this helper should be transparent
}

func (d *defaultUntarIOHelper) CopyN(dst io.Writer, n int64) (int64, error) {
	return io.CopyN(dst, d.tarReader, n) //nolint:wrapcheck // this helper should be transparent
}
