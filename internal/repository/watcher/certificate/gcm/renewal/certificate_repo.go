package renewal

import (
	"context"
	"errors"
	"fmt"

	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errNilCertificate            = errors.New("failed to update nil Certificate")
	errFailedToUpdateCertificate = errors.New("failed to update GCM Certificate")
)

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

	if err := r.kcpClient.Get(ctx, client.ObjectKeyFromObject(cert), cert); err != nil {
		return nil, fmt.Errorf("failed to get GCM Certificate %s-%s: %w", name, r.namespace, err)
	}

	return nil, nil
}

func (r *Repository) Update(ctx context.Context, cert *gcertv1alpha1.Certificate) error {
	if cert == nil {
		return errNilCertificate
	}

	if err := r.kcpClient.Update(ctx, cert); err != nil {
		return errFailedToUpdateCertificate
	}

	return nil
}
