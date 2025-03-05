package service

import (
	"context"
	"fmt"

	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/repository"
)

type ManifestService struct {
	descriptorProvider *provider.CachedDescriptorProvider
	manifestRepository *repository.ManifestRepository
}

func NewManifestService(client client.Client, descriptorProvider *provider.CachedDescriptorProvider) *ManifestService {
	return &ManifestService{
		descriptorProvider: descriptorProvider,
		manifestRepository: repository.NewManifestRepository(client),
	}
}

func (r *ManifestService) GetMandatoryManifests(ctx context.Context,
	template *v1beta2.ModuleTemplate,
) ([]v1beta2.Manifest, error) {
	descriptor, err := r.descriptorProvider.GetDescriptor(template)
	if err != nil {
		return nil, fmt.Errorf("not able to get descriptor from template: %w", err)
	}
	manifests, err := r.manifestRepository.ListByLabel(ctx, k8slabels.SelectorFromSet(k8slabels.Set{
		shared.IsMandatoryModule: "true",
	}))

	if client.IgnoreNotFound(err) != nil {
		return nil, fmt.Errorf("not able to list mandatory module manifests: %w", err)
	}

	return filterManifestsByAnnotation(manifests.Items, shared.FQDN, descriptor.GetName()), nil
}

func filterManifestsByAnnotation(manifests []v1beta2.Manifest,
	annotationKey, annotationValue string,
) []v1beta2.Manifest {
	filteredManifests := make([]v1beta2.Manifest, 0)
	for _, manifest := range manifests {
		if manifest.Annotations[annotationKey] == annotationValue {
			filteredManifests = append(filteredManifests, manifest)
		}
	}
	return filteredManifests
}
