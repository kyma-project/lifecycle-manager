package istiogateway

import (
	"context"
	"fmt"

	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Repository struct {
	reader client.Reader
}

func NewRepository(reader client.Reader) *Repository {
	return &Repository{
		reader: reader,
	}
}

func (r Repository) Get(ctx context.Context, name, namespace string) (*istioclientapiv1beta1.Gateway, error) {
	gateway := &istioclientapiv1beta1.Gateway{}
	if err := r.reader.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, gateway); err != nil {
		return nil, fmt.Errorf("failed to get istio gateway %s/%s: %w", namespace, name, err)
	}
	return gateway, nil
}
