package provider

/*
import (
	"fmt"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
	"github.com/kyma-project/lifecycle-manager/internal/repository/oci"

)

type Service struct {
	ocmRepository        *oci.Repository
}

func New(ocmRepository *oci.Repository) (*Service, error) {
	if ocmRepository == nil {
		return nil, fmt.Errorf("ociRepository must not be nil")
	}

	return &Service{
		ocmRepository: ocmRepository,
	}, nil
}

// GetDescriptor fetches the ComponentDescriptor for the given OCM identity.
// It returns an error if the identity is invalid or if fetching the descriptor fails.
func (s *Service) GetDescriptor(ocmi ocmidentity.Component) (*types.Descriptor, error) {

	cd, err := s.ocmRepository.GetComponentDescriptor(ocmi.Name(), ocmi.Version())

	if err != nil {
		return nil, fmt.Errorf("failed to get ComponentDescriptor for name=%q version=%q: %w",
			ocmi.Name(), ocmi.Version(), err)
	}

	return &types.Descriptor{ComponentDescriptor: cd}, nil
}
*/
