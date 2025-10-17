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
	ErrDecode           = errors.New("failed to decode component descriptor")
)

type OCIRepository interface {
	GetConfigFile(ctx context.Context, name, tag string) ([]byte, error)
	PullLayer(ctx context.Context, name, tag, digest string) (containerregistryv1.Layer, error)
}

type Service struct {
	ociRepository        OCIRepository
	extractFileFromLayer func(layer containerregistryv1.Layer, fileName string) ([]byte, error)
}

func NewService(ociRepository OCIRepository) (*Service, error) {
	if ociRepository == nil {
		return nil, fmt.Errorf("ociRepository must not be nil: %w", ErrInvalidArg)
	}

	return &Service{
		ociRepository:        ociRepository,
		extractFileFromLayer: defaultFileExtractor().extractFileFromLayer,
	}, nil
}

func commonErrMsg(ocmi ocmidentity.ComponentId) string {
	return fmt.Sprintf("ocm artifact with name=%q and version=%q",
		ocmi.Name(), ocmi.Version())
}

func (s *Service) GetComponentDescriptor(ctx context.Context, ocmi ocmidentity.ComponentId) (*types.Descriptor, error) {
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

	compdescBytes, err := s.extractFileFromLayer(layer, ComponentDescriptorFileName)
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
func deserialize(compdescBytes []byte, ocmi ocmidentity.ComponentId) (*compdesc.ComponentDescriptor, error) {
	desc, err := compdesc.Decode(compdescBytes)
	if err != nil {
		return nil, fmt.Errorf("%w fetched from %s: %w",
			ErrDecode, commonErrMsg(ocmi), err)
	}
	return desc, nil
}
