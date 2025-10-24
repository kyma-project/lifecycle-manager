package componentdescriptor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"
	"ocm.software/ocm/api/ocm/compdesc"
	"ocm.software/ocm/api/ocm/extensions/repositories/genericocireg"

	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
)

const (
	ComponentDescriptorFileName = compdesc.ComponentDescriptorFileName
)

var (
	ErrInvalidArg       = errors.New("invalid argument")
	ErrLayerNil         = errors.New("ComponentDescriptorLayer is nil in ComponentDescriptorConfig")
	ErrLayerDigestEmpty = errors.New("ComponentDescriptorLayer.Digest is empty in ComponentDescriptorConfig")
	ErrNotFoundInTar    = errors.New("not found in TAR archive")
	ErrTarTooLarge      = errors.New("entry in the TAR archive is too large")
	ErrLayerEmpty       = errors.New("layer content is empty")
)

type OCIRepository interface {
	GetConfigFile(ctx context.Context, name, tag string) ([]byte, error)
	PullLayer(ctx context.Context, name, tag, digest string) (containerregistryv1.Layer, error)
}

type fileExtractor interface {
	ExtractFileFromLayer(layer containerregistryv1.Layer, fileName string) ([]byte, error)
}

type Service struct {
	ociRepository OCIRepository
	fileExtractor fileExtractor
}

func NewService(ociRepository OCIRepository, fExt fileExtractor) (*Service, error) {
	if ociRepository == nil {
		return nil, fmt.Errorf("ociRepository must not be nil: %w", ErrInvalidArg)
	}

	if fExt == nil {
		return nil, fmt.Errorf("fileExtractor must not be nil: %w", ErrInvalidArg)
	}
	return &Service{
		ociRepository: ociRepository,
		fileExtractor: fExt,
	}, nil
}

func commonErrMsg(ocmId ocmidentity.ComponentId) string {
	return fmt.Sprintf("ocm artifact with name=%q and version=%q",
		ocmId.Name(), ocmId.Version())
}

func (s *Service) GetComponentDescriptor(ctx context.Context,
	ocmId ocmidentity.ComponentId,
) (*types.Descriptor, error) {
	compDescLayerDigest, err := s.getDescriptorLayerDigest(ctx, ocmId)
	if err != nil {
		return nil, err
	}

	compdescLayer, err := s.ociRepository.PullLayer(ctx, ocmId.Name(), ocmId.Version(), compDescLayerDigest)
	if err != nil {
		return nil, fmt.Errorf("failed to pull layer for ocm artifact with name=%q, version=%q and digest=%q: %w",
			ocmId.Name(), ocmId.Version(), compDescLayerDigest, err)
	}

	compdescBytes, err := s.fileExtractor.ExtractFileFromLayer(compdescLayer, ComponentDescriptorFileName)
	if err != nil {
		return nil,
			fmt.Errorf("failed to extract component descriptor from layer fetched from %s with digest=%q: %w",
				commonErrMsg(ocmId), compDescLayerDigest, err)
	}

	descriptor, err := types.Deserialize(compdescBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to get component descriptor for %s: %w", commonErrMsg(ocmId), err)
	}

	return &types.Descriptor{
		ComponentDescriptor: descriptor,
	}, nil
}

// getDescriptorLayerDigest retrieves the digest of the ComponentDescriptor layer.
func (s *Service) getDescriptorLayerDigest(ctx context.Context, ocmId ocmidentity.ComponentId) (string, error) {
	// Fetch the image config to get the ComponentDescriptor layer info
	configBytes, err := s.ociRepository.GetConfigFile(ctx, ocmId.Name(), ocmId.Version())
	if err != nil {
		return "", fmt.Errorf("failed to get config file for %s: %w", commonErrMsg(ocmId), err)
	}

	ocmArtifactConfig := genericocireg.ComponentDescriptorConfig{}
	err = json.Unmarshal(configBytes, &ocmArtifactConfig)
	if err != nil {
		return "",
			fmt.Errorf("failed to unmarshal config data into ComponentDescriptorConfig for %s: %w",
				commonErrMsg(ocmId), err)
	}

	if ocmArtifactConfig.ComponentDescriptorLayer == nil {
		return "", fmt.Errorf("%w for %s", ErrLayerNil, commonErrMsg(ocmId))
	}

	compDescLayerDigest := ocmArtifactConfig.ComponentDescriptorLayer.Digest
	if compDescLayerDigest == "" {
		return "",
			fmt.Errorf("%w for %s", ErrLayerDigestEmpty, commonErrMsg(ocmId))
	}

	return string(compDescLayerDigest), nil
}
