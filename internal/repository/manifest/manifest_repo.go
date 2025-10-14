package manifest

import (
	"context"
	"fmt"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type Repository struct {
	clnt      client.Client
	namespace string
}

func NewRepository(clnt client.Client, namespace string) *Repository {
	return &Repository{
		clnt:      clnt,
		namespace: namespace,
	}
}

func (r *Repository) DeleteAllForModule(ctx context.Context, moduleName string) error {
	err := r.clnt.DeleteAllOf(ctx, &v1beta2.Manifest{}, client.InNamespace(r.namespace),
		client.MatchingLabels{shared.ModuleName: moduleName})
	if err != nil {
		return fmt.Errorf("failed to delete all manifests for module %s: %w", moduleName, err)
	}
	return nil
}

func (r *Repository) ListAllForModule(ctx context.Context, moduleName string) (
	[]apimetav1.PartialObjectMetadata, error,
) {
	var manifestList apimetav1.PartialObjectMetadataList
	manifestList.SetGroupVersionKind(v1beta2.GroupVersion.WithKind("ManifestList"))

	if err := r.clnt.List(ctx, &manifestList, client.InNamespace(r.namespace),
		client.MatchingLabels{shared.ModuleName: moduleName}); err != nil {
		return nil, fmt.Errorf("failed to list Manifests for module %s: %w", moduleName, err)
	}
	return manifestList.Items, nil
}
