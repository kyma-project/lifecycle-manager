package repository

import (
	"context"
	"fmt"

	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type ManifestRepository struct {
	Client client.Client
}

func NewManifestRepository(client client.Client,
) *ManifestRepository {
	return &ManifestRepository{
		Client: client,
	}
}

func (m *ManifestRepository) RemoveManifests(ctx context.Context, manifests []v1beta2.Manifest) error {
	for _, manifest := range manifests {
		if err := m.Client.Delete(ctx, &manifest); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("not able to delete manifest %s/%s: %w", manifest.Namespace, manifest.Name, err)
		}
	}
	return nil
}

func (m *ManifestRepository) ListByLabel(ctx context.Context,
	labelSelector k8slabels.Selector,
) (*v1beta2.ManifestList, error) {
	manifestList := &v1beta2.ManifestList{}
	if err := m.Client.List(ctx, manifestList,
		&client.ListOptions{LabelSelector: labelSelector}); err != nil {
		return nil, fmt.Errorf("could not list ManifestList: %w", err)
	}
	return manifestList, nil
}
