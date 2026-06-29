package crd

import (
	"context"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Reader reads CustomResourceDefinitions by name.
type Reader interface {
	Get(ctx context.Context, name types.NamespacedName, obj client.Object, opts ...client.GetOption) error
}

// Repository reads CustomResourceDefinitions from the cluster the underlying client is connected to.
type Repository struct {
	reader Reader
}

func NewRepository(reader Reader) *Repository {
	return &Repository{reader: reader}
}

func (r *Repository) Get(ctx context.Context, name string) (*apiextensionsv1.CustomResourceDefinition, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := r.reader.Get(ctx, types.NamespacedName{Name: name}, crd); err != nil {
		return nil, fmt.Errorf("failed to get CRD %q: %w", name, err)
	}
	return crd, nil
}
