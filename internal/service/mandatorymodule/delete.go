package mandatorymodule

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/repository"
)

type MandatoryModuleDeletionService struct {
	ManifestRepository *repository.ManifestRepository
}

func NewMandatoryModuleDeletionService(client client.Client,
	descriptorProvider *provider.CachedDescriptorProvider,
) *MandatoryModuleDeletionService {
	return &MandatoryModuleDeletionService{
		ManifestRepository: repository.NewManifestRepository(client, descriptorProvider),
	}
}

func (s *MandatoryModuleDeletionService) DeleteMandatoryModules(ctx context.Context,
	template *v1beta2.ModuleTemplate,
) (bool, error) {
	manifests, err := s.ManifestRepository.GetMandatoryManifests(ctx, template)
	if err != nil {
		return false, fmt.Errorf("failed to get MandatoryModuleManifests: %w", err)
	}

	if len(manifests) == 0 {
		return true, nil
	}

	if err := s.ManifestRepository.RemoveManifests(ctx, manifests); err != nil {
		return false, fmt.Errorf("failed to remove MandatoryModule Manifest: %w", err)
	}

	return false, nil
}
