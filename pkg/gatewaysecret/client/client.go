package client

import (
	"context"
	"fmt"

	certmanagerclientv1 "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/typed/certmanager/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"

	"github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"

	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	apicorev1 "k8s.io/api/core/v1"
)

const (
	istioNamespace    = "istio-system"
	gatewaySecretName = "klm-istio-gateway"
	kcpCACertName     = "klm-watcher-serving"
)

type GatewaySecretRotationClient struct {
	certificateInterface certmanagerclientv1.CertificateInterface
	secretInterface      corev1.SecretInterface
}

func NewGatewaySecretRotationClient(config *rest.Config) *GatewaySecretRotationClient {
	return &GatewaySecretRotationClient{
		certificateInterface: versioned.NewForConfigOrDie(config).CertmanagerV1().Certificates(istioNamespace),
		secretInterface:      kubernetes.NewForConfigOrDie(config).CoreV1().Secrets(istioNamespace),
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
	secret, err := c.secretInterface.Get(ctx, gatewaySecretName, apimetav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway secret %s: %w", gatewaySecretName, err)
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
	if gatewaySecret.Name != gatewaySecretName {
		return fmt.Errorf("invalid gateway secret name %s, expected %s", gatewaySecret.Name, gatewaySecretName)
	}

	if gatewaySecret.Namespace != istioNamespace {
		return fmt.Errorf("invalid gateway secret namespace %s, expected %s", gatewaySecret.Namespace, istioNamespace)
	}

	return nil
}
