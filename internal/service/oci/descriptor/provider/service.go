package provider

import (
	"context"
	"net/url"
	"ocm.software/ocm/api/ocm/compdesc"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
	"github.com/kyma-project/lifecycle-manager/internal/service/configuration"
	"ocm.software/ocm/api/ocm/extensions/accessmethods/ociartifact"
	"ocm.software/ocm/api/ocm/extensions/repositories/genericocireg"
)

type Service struct {
	ConfigurationService *configuration.Service
}

func (c *Service) GetDescriptor(template *v1beta2.ModuleTemplate) (*types.Descriptor, error) {
	ociRegistryURL, err := c.ConfigurationService.GetOCIRegistryURL()
	if err != nil {
		return nil, err
	}

	genericociregistry := genericocireg.NewRepositorySpec(null, null)
	cpiRepo, err := genericociregistry.Repository()
	if err != nil {
		return nil, err
	}

	return nil, nil //TODO: implement
}
