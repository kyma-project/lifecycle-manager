package gatewaysecret

import (
	"context"
	"fmt"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SecretManager interface {
	FindGatewaySecret(ctx context.Context) (*apicorev1.Secret, error)
	GetRootCACertificate(ctx context.Context) (certmanagerv1.Certificate, error)
	Create(ctx context.Context, secret *apicorev1.Secret) error
	Update(ctx context.Context, secret *apicorev1.Secret) error
}

type SecretManagerImpl struct {
	kcpClient client.Client
}

func NewSecretManagerImpl(kcpClient client.Client) *SecretManagerImpl {
	return &SecretManagerImpl{kcpClient: kcpClient}
}

func (sm *SecretManagerImpl) FindGatewaySecret(ctx context.Context) (*apicorev1.Secret, error) {
	return GetGatewaySecret(ctx, sm.kcpClient)
}

func (sm *SecretManagerImpl) Create(ctx context.Context, secret *apicorev1.Secret) error {
	sm.updateLastModifiedAt(secret)
	if err := sm.kcpClient.Create(ctx, secret); err != nil {
		return fmt.Errorf("failed to create secret %s: %w", secret.Name, err)
	}
	return nil
}

func (sm *SecretManagerImpl) Update(ctx context.Context, secret *apicorev1.Secret) error {
	sm.updateLastModifiedAt(secret)
	if err := sm.kcpClient.Update(ctx, secret); err != nil {
		return fmt.Errorf("failed to update secret %s: %w", secret.Name, err)
	}
	return nil
}

func (sm *SecretManagerImpl) GetRootCACertificate(ctx context.Context) (certmanagerv1.Certificate, error) {
	caCert := certmanagerv1.Certificate{}
	if err := sm.kcpClient.Get(ctx,
		client.ObjectKey{Namespace: istioNamespace, Name: kcpCACertName},
		&caCert); err != nil {
		return certmanagerv1.Certificate{}, fmt.Errorf("failed to get CA certificate: %w", err)
	}
	return caCert, nil
}

func (sm *SecretManagerImpl) updateLastModifiedAt(secret *apicorev1.Secret) {
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[LastModifiedAtAnnotation] = apimetav1.Now().Format(time.RFC3339)
}

func NewGatewaySecret(rootSecret *apicorev1.Secret) *apicorev1.Secret {
	gwSecret := &apicorev1.Secret{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: apicorev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      gatewaySecretName,
			Namespace: istioNamespace,
		},
		Data: map[string][]byte{
			"tls.crt": rootSecret.Data["tls.crt"],
			"tls.key": rootSecret.Data["tls.key"],
			"ca.crt":  rootSecret.Data["ca.crt"],
		},
	}
	return gwSecret
}

func GetGatewaySecret(ctx context.Context, clnt client.Client) (*apicorev1.Secret, error) {
	secret := &apicorev1.Secret{}
	if err := clnt.Get(ctx, client.ObjectKey{
		Name:      gatewaySecretName,
		Namespace: istioNamespace,
	}, secret); err != nil {
		return nil, fmt.Errorf("failed to get secret %s: %w", gatewaySecretName, err)
	}
	return secret, nil
}
