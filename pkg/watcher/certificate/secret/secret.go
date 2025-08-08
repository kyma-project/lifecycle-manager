package secret

import (
	"context"
	"errors"
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	CaCertKey        = "ca.crt"
	TlsCertKey       = "tls.crt"
	TlsPrivateKeyKey = "tls.key"
)

var ErrSecretDataMissing = errors.New("secret data is missing")

type CertificateSecretData struct {
	TlsCert, TlsKey []byte
}

type GatewaySecretData struct {
	CaCert []byte
}

func NewGatewaySecretData(secret *apicorev1.Secret) (*GatewaySecretData, error) {
	if secret == nil || secret.Data == nil || secret.Data[CaCertKey] == nil {
		return nil, fmt.Errorf("error in gateway secret %w", ErrSecretDataMissing)
	}
	return &GatewaySecretData{
		CaCert: secret.Data[CaCertKey],
	}, nil
}

func NewCertificateSecretData(secret *apicorev1.Secret) (*CertificateSecretData, error) {
	if secret == nil || secret.Data == nil || secret.Data[TlsCertKey] == nil || secret.Data[TlsPrivateKeyKey] == nil {
		return nil, fmt.Errorf("error in certificate secret %w", ErrSecretDataMissing)
	}

	return &CertificateSecretData{
		TlsCert: secret.Data[TlsCertKey],
		TlsKey:  secret.Data[TlsPrivateKeyKey],
	}, nil
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

	err := s.kcpClient.Delete(ctx, secret)
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to delete secret %s-%s: %w", name, namespace, err)
	}

	return nil
}
