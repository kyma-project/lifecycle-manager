package data

import (
	"errors"
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
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
