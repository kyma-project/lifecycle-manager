package componentdescriptor

import (
	"errors"
	"fmt"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
	"ocm.software/ocm/api/ocm/compdesc"
)

var (
	ErrInvalidArg = errors.New("invalid argument")
)

type OCIRepository interface {
	GetComponentDescriptor(ocmName, version string) (*compdesc.ComponentDescriptor, error)
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

func (s *Service) GetComponentDescriptor(ocmi ocmidentity.Component) (*types.Descriptor, error) {
	cd, err := s.ociRepository.GetComponentDescriptor(ocmi.Name(), ocmi.Version())
	if err != nil {
		return nil, fmt.Errorf(
			"error geting Component Descriptor for name=%s and version=%s: %w",
			ocmi.Name(), ocmi.Version(), err)
	}
	return &types.Descriptor{
		ComponentDescriptor: cd,
	}, nil
}
