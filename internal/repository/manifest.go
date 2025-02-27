package repository

import (
	"context"
	"fmt"

	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
)

type ManifestRepository struct {
	Client             client.Client
	DescriptorProvider *provider.CachedDescriptorProvider
}

func (r *ManifestRepository) GetCorrespondingManifests(ctx context.Context,
	template *v1beta2.ModuleTemplate) ([]v1beta2.Manifest, error) {
	manifests := &v1beta2.ManifestList{}
	descriptor, err := r.DescriptorProvider.GetDescriptor(template)
	if err != nil {
		return nil, fmt.Errorf("not able to get descriptor from template: %w", err)
	}
	if err := r.Client.List(ctx, manifests, &client.ListOptions{
		Namespace:     template.Namespace,
		LabelSelector: k8slabels.SelectorFromSet(k8slabels.Set{shared.IsMandatoryModule: "true"}),
	}); client.IgnoreNotFound(err) != nil {
		return nil, fmt.Errorf("not able to list mandatory module manifests: %w", err)
	}

	return filterManifestsByAnnotation(manifests.Items, shared.FQDN, descriptor.GetName()), nil
}

func filterManifestsByAnnotation(manifests []v1beta2.Manifest,
	annotationKey, annotationValue string) []v1beta2.Manifest {
	filteredManifests := make([]v1beta2.Manifest, 0)
	for _, manifest := range manifests {
		if manifest.Annotations[annotationKey] == annotationValue {
			filteredManifests = append(filteredManifests, manifest)
		}
	}
	return filteredManifests
}
