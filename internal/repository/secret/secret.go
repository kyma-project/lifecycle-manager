package secret

import (
	"context"
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type kcpClient interface {
	Get(ctx context.Context,
		key client.ObjectKey,
		obj client.Object,
		opts ...client.GetOption,
	) error
	Delete(ctx context.Context,
		obj client.Object,
		opts ...client.DeleteOption,
	) error
}

type Secret struct {
	kcpClient kcpClient
}

func NewCertificateSecretClient(kcpClient kcpClient) *Secret {
	return &Secret{
		kcpClient,
	}
}

func (s *Secret) Get(ctx context.Context,
	name string,
	namespace string,
) (*apicorev1.Secret, error) {
	secret := &apicorev1.Secret{}
	err := s.kcpClient.Get(ctx,
		client.ObjectKey{Name: name, Namespace: namespace}, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s-%s: %w", name, namespace, err)
	}

	return secret, nil
}

func (s *Secret) Delete(ctx context.Context,
	name string,
	namespace string,
) error {
	secret := &apicorev1.Secret{}
	secret.SetName(name)
	secret.SetNamespace(namespace)

	if err := s.kcpClient.Delete(ctx, secret); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to delete secret %s-%s: %w", name, namespace, err)
	}

	return nil
}
