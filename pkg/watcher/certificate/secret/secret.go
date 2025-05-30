package secret

import (
	"context"
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	CaCertKey        = "ca.crt"
	TlsCertKey       = "tls.crt"
	TlsPrivateKeyKey = "tls.key"
)

type CertificateSecretData struct {
	TlsCert, TlsKey []byte
}

type GatewaySecretData struct {
	CaCert []byte
}

func NewGatewaySecretData(secret *apicorev1.Secret) *GatewaySecretData {
	return &GatewaySecretData{
		CaCert: secret.Data[CaCertKey],
	}
}

func NewCertificateSecretData(secret *apicorev1.Secret) *CertificateSecretData {
	return &CertificateSecretData{
		TlsCert: secret.Data[TlsCertKey],
		TlsKey:  secret.Data[TlsPrivateKeyKey],
	}
}

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

type CertificateSecretClient struct {
	kcpClient kcpClient
}

func NewCertificateSecretClient(kcpClient kcpClient) *CertificateSecretClient {
	return &CertificateSecretClient{
		kcpClient,
	}
}

func (s *CertificateSecretClient) Get(ctx context.Context,
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

func (s *CertificateSecretClient) Delete(ctx context.Context,
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
