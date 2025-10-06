package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

const (
	kcpCACertName = "klm-watcher-serving"
)

var errInvalidGatewaySecret = errors.New("invalid gateway secret")

type CertificateInterface interface {
	GetValidity(ctx context.Context, name string) (time.Time, time.Time, error)
}

type GatewaySecretRotationClient struct {
	certificateInterface CertificateInterface
	secretInterface      k8scorev1.SecretInterface
}

func NewGatewaySecretRotationClient(
	config *rest.Config,
	certificateInterface CertificateInterface,
) *GatewaySecretRotationClient {
	return &GatewaySecretRotationClient{
		certificateInterface: certificateInterface,
		secretInterface:      kubernetes.NewForConfigOrDie(config).CoreV1().Secrets(shared.IstioNamespace),
	}
}

func (c *GatewaySecretRotationClient) GetWatcherServingCertValidity(ctx context.Context) (time.Time, time.Time, error) {
	notBefore, notAfter, err := c.certificateInterface.GetValidity(ctx, kcpCACertName)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("failed to get watcher serving cert validity: %w", err)
	}

	return notBefore, notAfter, nil
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
		return fmt.Errorf(
			"expected name %s to be %s: %w",
			gatewaySecret.Name,
			shared.GatewaySecretName,
			errInvalidGatewaySecret,
		)
	}

	if gatewaySecret.Namespace != shared.IstioNamespace {
		return fmt.Errorf(
			"expected namespace %s to be %s: %w",
			gatewaySecret.Namespace,
			shared.IstioNamespace,
			errInvalidGatewaySecret,
		)
	}

	return nil
}
