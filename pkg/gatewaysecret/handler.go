package gatewaysecret

import (
	"context"
	"errors"
	"fmt"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	certmanagerclientv1 "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/typed/certmanager/v1"
	"github.com/go-logr/logr"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"

	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

const (
	LastModifiedAtAnnotation = "lastModifiedAt"
	kcpCACertName            = "klm-watcher-serving"
)

var errCouldNotGetLastModifiedAt = errors.New("getting lastModifiedAt time failed")

type Handler struct {
	certificateClient certmanagerclientv1.CertificateInterface
	kcpSecretClient   corev1.SecretInterface
	log               logr.Logger
}

func NewGatewaySecretHandler(config *rest.Config, log logr.Logger) *Handler {
	return &Handler{
		certificateClient: versioned.NewForConfigOrDie(config).CertmanagerV1().Certificates(istioNamespace),
		kcpSecretClient:   kubernetes.NewForConfigOrDie(config).CoreV1().Secrets(istioNamespace),
		log:               log,
	}
}

func (h *Handler) ManageGatewaySecret(ctx context.Context, rootSecret *apicorev1.Secret) error {
	gwSecret, err := h.findGatewaySecret(ctx)
	if util.IsNotFound(err) {
		// Create gateway secret with the root secret data
		return h.createGatewaySecret(ctx, rootSecret)
	}
	if err != nil {
		return err
	}

	caCert, err := h.GetRootCACertificate(ctx)
	if err != nil {
		return err
	}
	if !RequiresUpdate(gwSecret, caCert) {
		return nil
	}

	CopySecretData(rootSecret, gwSecret)
	return h.Update(ctx, gwSecret)
}

func (h *Handler) findGatewaySecret(ctx context.Context) (*apicorev1.Secret, error) {
	secret, err := h.kcpSecretClient.Get(ctx, gatewaySecretName, apimetav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s: %w", gatewaySecretName, err)
	}
	return secret, nil
}

func (h *Handler) GetRootCACertificate(ctx context.Context) (*certmanagerv1.Certificate, error) {
	caCert, err := h.certificateClient.Get(ctx, kcpCACertName, apimetav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get CA certificate: %w", err)
	}
	return caCert, nil
}

func (h *Handler) Create(ctx context.Context, secret *apicorev1.Secret) error {
	setLastModifiedToNow(secret)
	if _, err := h.kcpSecretClient.Create(ctx, secret, apimetav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create secret %s: %w", secret.Name, err)
	}
	return nil
}

func (h *Handler) Update(ctx context.Context, secret *apicorev1.Secret) error {
	setLastModifiedToNow(secret)
	if _, err := h.kcpSecretClient.Update(ctx, secret, apimetav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to update secret %s: %w", secret.Name, err)
	}
	return nil
}

func (h *Handler) createGatewaySecret(ctx context.Context, secret *apicorev1.Secret) error {
	return h.Create(ctx, NewGatewaySecret(secret))
}

func RequiresUpdate(gwSecret *apicorev1.Secret, caCert *certmanagerv1.Certificate) bool {
	if gwSecretLastModifiedAt, err := ParseLastModifiedTime(gwSecret); err == nil {
		if caCert.Status.NotBefore != nil && gwSecretLastModifiedAt.After(caCert.Status.NotBefore.Time) {
			return false
		}
	}
	return true
}

func CopySecretData(from, to *apicorev1.Secret) {
	to.Data[tlsCrt] = from.Data[tlsCrt]
	to.Data[tlsKey] = from.Data[tlsKey]
	to.Data[caCrt] = from.Data[caCrt]
}

func setLastModifiedToNow(secret *apicorev1.Secret) {
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[LastModifiedAtAnnotation] = apimetav1.Now().Format(time.RFC3339)
}
