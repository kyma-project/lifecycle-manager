package renewal

import (
	"context"
	"errors"
	"fmt"

	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrNilCertificate = errors.New("failed to update nil Certificate")

type Repository struct {
	kcpClient client.Client
	namespace string
}

func NewRepository(kcpClient client.Client, namespace string) *Repository {
	return &Repository{
		kcpClient: kcpClient,
		namespace: namespace,
	}
}

func (r *Repository) Get(ctx context.Context, name string) (*gcertv1alpha1.Certificate, error) {
	cert := &gcertv1alpha1.Certificate{}
	cert.SetName(name)
	cert.SetNamespace(r.namespace)

	err := r.kcpClient.Get(ctx, client.ObjectKeyFromObject(cert), cert)
	if err != nil {
		return nil, fmt.Errorf("failed to get GCM Certificate %s-%s: %w", name, r.namespace, err)
	}

	return cert, nil
}

func (r *Repository) Update(ctx context.Context, cert *gcertv1alpha1.Certificate) error {
	if cert == nil {
		return ErrNilCertificate
	}

	err := r.kcpClient.Update(ctx, cert)
	if err != nil {
		return fmt.Errorf("failed to update GCM Certificate %s-%s: %w", cert.Name, cert.Namespace, err)
	}

	return nil
}
