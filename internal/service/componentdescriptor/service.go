package componentdescriptor

import (
	"errors"
	"fmt"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
	"ocm.software/ocm/api/ocm/compdesc"
)

var (
	ErrInvalidArg = errors.New("invalid argument")
)

type OCIRepository interface {
	GetComponentDescriptor(name, version string) (*compdesc.ComponentDescriptor, error)
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

func (s *Service) GetComponentDescriptor(ociComponentName, componentVersion string) (*types.Descriptor, error) {
	componentDescriptor, err := s.ociRepository.GetComponentDescriptor(ociComponentName, componentVersion)
	if err != nil {
		return nil, fmt.Errorf("error geting Component Descriptor for name=%s and version=%s: %w",
			ociComponentName, componentVersion, err)
	}
	return &types.Descriptor{
		ComponentDescriptor: componentDescriptor,
	}, nil
}
