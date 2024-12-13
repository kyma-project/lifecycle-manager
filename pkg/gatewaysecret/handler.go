package gatewaysecret

import (
	"context"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type Client interface {
	GetWatcherServingCert(ctx context.Context) (*v1.Certificate, error)
	GetGatewaySecret(ctx context.Context) (*apicorev1.Secret, error)
	CreateGatewaySecret(ctx context.Context, secret *apicorev1.Secret) error
	UpdateGatewaySecret(ctx context.Context, secret *apicorev1.Secret) error
}

type TimeParserFunc func(secret *apicorev1.Secret) (time.Time, error)

type Handler struct {
	client                Client
	parseLastModifiedTime TimeParserFunc
}

func NewGatewaySecretHandler(client Client, timeParserFunc TimeParserFunc) *Handler {
	return &Handler{
		client:                client,
		parseLastModifiedTime: timeParserFunc,
	}
}

const (
	tlsCrt     = "tls.crt"
	tlsKey     = "tls.key"
	caCrt      = "ca.crt"
	secretKind = "Secret"
)

func (h *Handler) ManageGatewaySecret(ctx context.Context, rootSecret *apicorev1.Secret) error {
	gwSecret, err := h.client.GetGatewaySecret(ctx)
	if util.IsNotFound(err) {
		return h.createGatewaySecretFromRootSecret(ctx, rootSecret)
	} else if err != nil {
		return err
	}

	caCert, err := h.client.GetWatcherServingCert(ctx)
	if err != nil {
		return err
	}

	if h.requiresUpdate(gwSecret, caCert) {
		copyDataFromRootSecret(gwSecret, rootSecret)
		setLastModifiedToNow(gwSecret)

		return h.client.UpdateGatewaySecret(ctx, gwSecret)
	}

	return nil
}

func (h *Handler) createGatewaySecretFromRootSecret(ctx context.Context, rootSecret *apicorev1.Secret) error {
	newSecret := &apicorev1.Secret{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       secretKind,
			APIVersion: apicorev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      shared.GatewaySecretName,
			Namespace: shared.IstioNamespace,
		},
	}

	copyDataFromRootSecret(newSecret, rootSecret)
	setLastModifiedToNow(newSecret)

	return h.client.CreateGatewaySecret(ctx, newSecret)
}

func (h *Handler) requiresUpdate(gwSecret *apicorev1.Secret, caCert *certmanagerv1.Certificate) bool {
	// If the last modified time of the gateway secret is after the notBefore time of the CA certificate,
	// then we don't need to update the gateway secret
	if lastModified, err := h.parseLastModifiedTime(gwSecret); err == nil {
		if caCert.Status.NotBefore != nil && lastModified.After(caCert.Status.NotBefore.Time) {
			return false
		}
	}
	return true
}

func setLastModifiedToNow(secret *apicorev1.Secret) {
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[shared.LastModifiedAtAnnotation] = apimetav1.Now().Format(time.RFC3339)
}

func copyDataFromRootSecret(secret *apicorev1.Secret, rootSecret *apicorev1.Secret) {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[tlsCrt] = rootSecret.Data[tlsCrt]
	secret.Data[tlsKey] = rootSecret.Data[tlsKey]
	secret.Data[caCrt] = rootSecret.Data[caCrt]
}
