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
	MaxDescriptorSizeBytes      = 100 * 1024 // 100KiB
	TarReadChunkSize            = 1024       // 1KiB
	ComponentDescriptorFileName = "component-descriptor.yaml"
)

var (
	ErrInvalidArg       = errors.New("invalid argument")
	ErrLayerNil         = errors.New("ComponentDescriptorLayer is nil in ComponentDescriptorConfig")
	ErrLayerDigestEmpty = errors.New("ComponentDescriptorLayer.Digest is empty in ComponentDescriptorConfig")
	ErrNotFoundInTar    = errors.New("not found in TAR archive")
)

type OCIRepository interface {
	GetConfigFile(ctx context.Context, name, tag string) ([]byte, error)
	PullLayer(ctx context.Context, name, tag, digest string) (containerregistryv1.Layer, error)
}

// Helper interface to simplify logic.
type FileExtractor interface {
	ExtractFile(layer containerregistryv1.Layer, fileName string) ([]byte, error)
}

type Service struct {
	ociRepository OCIRepository
	fileExtractor FileExtractor
}

func NewService(ociRepository OCIRepository) (*Service, error) {
	if ociRepository == nil {
		return nil, fmt.Errorf("ociRepository must not be nil: %w", ErrInvalidArg)
	}

	return &Service{
		ociRepository: ociRepository,
		fileExtractor: &defaultFileExtractor{},
	}, nil
}

func (s *Service) GetComponentDescriptor(ctx context.Context, ocmi ocmidentity.Component) (*types.Descriptor, error) {
	// {"componentDescriptorLayer":{"mediaType":"application/vnd.ocm.software.component-descriptor.v2+yaml+tar","digest":"sha256:4e51d8f80b88bdbd208e6e22314376a0d5212026bf3054f8ef79d43250e5182b","size":4608}}
	// ref := fmt.Sprintf("k3d-kcp-registry.localhost:5000/component-descriptors/%s:%s", name, version)
	commonErrMsg := func() string {
		return fmt.Sprintf("ocm artifact with name=%q and version=%q",
			ocmi.Name(), ocmi.Version())
	}

	// Fetch the image config to get the ComponentDescriptor layer info
	configBytes, err := s.ociRepository.GetConfigFile(ctx, ocmi.Name(), ocmi.Version())
	if err != nil {
		return nil, fmt.Errorf("failed to get config file for %s: %w", commonErrMsg(), err)
	}

	ocmArtifactConfig := genericocireg.ComponentDescriptorConfig{}

	err = json.Unmarshal(configBytes, &ocmArtifactConfig)
	if err != nil {
		return nil,
			fmt.Errorf("failed to unmarshal config data into ComponentDescriptorConfig for %s: %w",
				commonErrMsg(), err)
	}

	if ocmArtifactConfig.ComponentDescriptorLayer == nil {
		return nil, fmt.Errorf("%w for %s", ErrLayerNil, commonErrMsg())
	}

	compDescLayerDigest := ocmArtifactConfig.ComponentDescriptorLayer.Digest
	if string(compDescLayerDigest) == "" {
		return nil,
			fmt.Errorf("%w for %s", ErrLayerDigestEmpty, commonErrMsg())
	}

	layer, err := s.ociRepository.PullLayer(ctx, ocmi.Name(), ocmi.Version(), string(compDescLayerDigest))
	if err != nil {
		return nil, fmt.Errorf("failed to pull layer for ocm artifact with name=%q, version=%q and digest=%q: %w",
			ocmi.Name(), ocmi.Version(), string(compDescLayerDigest), err)
	}

	compdescBytes, err := s.fileExtractor.ExtractFile(layer, ComponentDescriptorFileName)
	if err != nil {
		return nil,
			fmt.Errorf("failed to extract component descriptor from layer fetched from %s with digest=%q: %w",
				commonErrMsg(), string(compDescLayerDigest), err)
	}
	descriptor, err := compdesc.Decode(compdescBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decode component descriptor fetched from %s: %w",
			commonErrMsg(), err)
	}

	return &types.Descriptor{
		ComponentDescriptor: descriptor,
	}, nil
}

type defaultFileExtractor struct{}

func (d *defaultFileExtractor) ExtractFile(layer containerregistryv1.Layer, fileName string) ([]byte, error) {
	layerReader, err := layer.Uncompressed()
	if err != nil {
		return nil, err //nolint:wrapcheck // let the caller wrap
	}
	defer layerReader.Close()

	layerBytes, err := io.ReadAll(layerReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read layer data: %w", err)
	}

	compdescBytes, err := unTar(layerBytes, fileName)
	if err != nil {
		digest, derr := layer.Digest()
		if derr != nil {
			err = errors.Join(err, derr)
		}
		return nil,
			fmt.Errorf("failed to extract data from TAR archive for file=%q in layer with digest=%q: %w",
				fileName, digest, err)
	}

	return compdescBytes, nil
}

// unTar extracts the file with expectedName from the given tarBytes and returns its content.
func unTar(tarBytes []byte, expectedName string) ([]byte, error) {
	treader := tar.NewReader(bytes.NewReader(tarBytes))

	for {
		hdr, err := treader.Next()
		if err == io.EOF {
			break // end of archive
		}
		if err != nil {
			return nil, err //nolint:wrapcheck // will be wrapped by caller
		}

		if hdr.Name == expectedName {
			var buf bytes.Buffer
			maxSize := hdr.Size
			if maxSize <= 0 {
				maxSize = MaxDescriptorSizeBytes // use default max if size is not set or invalid
			}
			for buf.Len() < int(maxSize) {
				if _, err := io.CopyN(&buf, treader, TarReadChunkSize); err != nil {
					if errors.Is(err, io.EOF) {
						break
					}
					return nil, err //nolint:wrapcheck // will be wrapped by caller
				}
			}

			return buf.Bytes(), nil
		}
	}

	return nil, fmt.Errorf("%s %w", expectedName, ErrNotFoundInTar)
}
