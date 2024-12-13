package client

import (
	"context"
	"fmt"

	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	certmanagerclientv1 "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/typed/certmanager/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

const (
	kcpCACertName = "klm-watcher-serving"
)

type GatewaySecretRotationClient struct {
	certificateInterface certmanagerclientv1.CertificateInterface
	secretInterface      corev1.SecretInterface
}

func NewGatewaySecretRotationClient(config *rest.Config) *GatewaySecretRotationClient {
	return &GatewaySecretRotationClient{
		certificateInterface: versioned.NewForConfigOrDie(config).CertmanagerV1().Certificates(shared.IstioNamespace),
		secretInterface:      kubernetes.NewForConfigOrDie(config).CoreV1().Secrets(shared.IstioNamespace),
	}
}

func (c *GatewaySecretRotationClient) GetWatcherServingCert(ctx context.Context) (*v1.Certificate, error) {
	caCert, err := c.certificateInterface.Get(ctx, kcpCACertName, apimetav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get CA certificate %s: %w", kcpCACertName, err)
	}

	return caCert, nil
}

func (c *GatewaySecretRotationClient) GetGatewaySecret(ctx context.Context) (*apicorev1.Secret, error) {
	secret, err := c.secretInterface.Get(ctx, shared.GatewaySecretName, apimetav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway secret %s: %w", shared.GatewaySecretName, err)
	}

	return secret, nil
}

func (c *GatewaySecretRotationClient) CreateGatewaySecret(ctx context.Context, gatewaySecret *apicorev1.Secret) error {
	if err := ensureGatewaySecret(gatewaySecret); err != nil {
		return err
	}

	if _, err := c.secretInterface.Create(ctx, gatewaySecret, apimetav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create gateway secret %s: %w", gatewaySecret.Name, err)
	}

	return nil
}

func (c *GatewaySecretRotationClient) UpdateGatewaySecret(ctx context.Context, gatewaySecret *apicorev1.Secret) error {
	if err := ensureGatewaySecret(gatewaySecret); err != nil {
		return err
	}

	if _, err := c.secretInterface.Update(ctx, gatewaySecret, apimetav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to update gateway secret %s: %w", gatewaySecret.Name, err)
	}

	return nil
}

func ensureGatewaySecret(gatewaySecret *apicorev1.Secret) error {
	if gatewaySecret.Name != shared.GatewaySecretName {
		return fmt.Errorf("invalid gateway secret name %s, expected %s", gatewaySecret.Name, shared.GatewaySecretName)
	}

	if gatewaySecret.Namespace != shared.IstioNamespace {
		return fmt.Errorf("invalid gateway secret namespace %s, expected %s", gatewaySecret.Namespace, shared.IstioNamespace)
	}

	return nil
}
