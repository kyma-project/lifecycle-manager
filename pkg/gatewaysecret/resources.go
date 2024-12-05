package gatewaysecret

import (
	"context"
	"fmt"
	"time"

	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	secretKind        = "Secret"
	gatewaySecretName = "klm-istio-gateway" //nolint:gosec // gatewaySecretName is not a credential
	istioNamespace    = "istio-system"

	tlsCrt = "tls.crt"
	tlsKey = "tls.key"
	caCrt  = "ca.crt"
)

func NewGatewaySecret(rootSecret *apicorev1.Secret) *apicorev1.Secret {
	return &apicorev1.Secret{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       secretKind,
			APIVersion: apicorev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      gatewaySecretName,
			Namespace: istioNamespace,
		},
		Data: map[string][]byte{
			tlsCrt: rootSecret.Data[tlsCrt],
			tlsKey: rootSecret.Data[tlsKey],
			caCrt:  rootSecret.Data[caCrt],
		},
	}
}

func GetGatewaySecret(ctx context.Context, clnt client.Client) (*apicorev1.Secret, error) {
	secret := &apicorev1.Secret{}
	if err := clnt.Get(ctx, client.ObjectKey{
		Name:      gatewaySecretName,
		Namespace: istioNamespace,
	}, secret); err != nil {
		return nil, fmt.Errorf("failed to get gateway secret %s: %w", gatewaySecretName, err)
	}
	return secret, nil
}

func ParseLastModifiedTime(secret *apicorev1.Secret) (time.Time, error) {
	if gwSecretLastModifiedAtValue, ok := secret.Annotations[LastModifiedAtAnnotation]; ok {
		if gwSecretLastModifiedAt, err := time.Parse(time.RFC3339, gwSecretLastModifiedAtValue); err == nil {
			return gwSecretLastModifiedAt, nil
		}
	}
	return time.Time{}, errCouldNotGetLastModifiedAt
}
