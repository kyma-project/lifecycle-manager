package cabundle

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/gatewaysecret"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var (
	ErrCACertificateNotReady           = errors.New("watcher-serving ca certificate is not ready")
	ErrServerCertificateParsingFailure = errors.New("failed to parse server certificate from gateway secret")
)

type Bundler interface {
	Bundle(bundle *[]byte, cert []byte) (bool, error)
	DropExpiredCerts(bundle *[]byte) (bool, error)
}

type GatewaySecretMetrics interface {
	ServerCertificateCloseToExpiry(set bool)
}

type Handler struct {
	client                      gatewaysecret.Client
	serverCertSwitchGracePeriod time.Duration
	serverCertExpiryWindow      time.Duration
	bundler                     Bundler
	metrics                     GatewaySecretMetrics
}

func NewGatewaySecretHandler(client gatewaysecret.Client,
	serverCertSwitchGracePeriod time.Duration,
	serverCertExpiryWindow time.Duration,
	bundler Bundler,
	metrics GatewaySecretMetrics,
) *Handler {
	return &Handler{
		client:                      client,
		serverCertSwitchGracePeriod: serverCertSwitchGracePeriod,
		serverCertExpiryWindow:      serverCertExpiryWindow,
		bundler:                     bundler,
		metrics:                     metrics,
	}
}

func (h *Handler) ManageGatewaySecret(ctx context.Context, rootSecret *apicorev1.Secret) error {
	notBefore, _, err := h.client.GetWatcherServingCertValidity(ctx)
	if err != nil {
		return err
	}
	if notBefore.IsZero() {
		return ErrCACertificateNotReady
	}

	gwSecret, err := h.client.GetGatewaySecret(ctx)
	if util.IsNotFound(err) {
		return h.createGatewaySecretFromRootSecret(ctx, rootSecret)
	} else if err != nil {
		return err
	}

	bundled, err := h.bundleCACerts(gwSecret, rootSecret)
	if err != nil {
		return err
	}

	if bundled {
		setCaBundleTimeAnnotationToNow(gwSecret)
	}

	err = h.dropExpiredCertsFromBundle(gwSecret)
	if err != nil {
		return err
	}

	if h.requiresCertSwitching(notBefore) {
		logf.FromContext(ctx).
			V(log.InfoLevel).
			Info("Switching gateway secret tls.crt",
				"caNotBefore", notBefore.Format(time.RFC3339),
				"serverCertSwitchGracePeriod", h.serverCertSwitchGracePeriod,
			)
		switchCertificate(gwSecret, rootSecret)
	}

	if err := h.updateServerCertExpiryMetric(gwSecret); err != nil {
		return err
	}

	return h.client.UpdateGatewaySecret(ctx, gwSecret)
}

func (h *Handler) createGatewaySecretFromRootSecret(ctx context.Context,
	rootSecret *apicorev1.Secret,
) error {
	newSecret := &apicorev1.Secret{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       gatewaysecret.SecretKind,
			APIVersion: apicorev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      shared.GatewaySecretName,
			Namespace: shared.IstioNamespace,
		},
	}

	newSecret.Data = make(map[string][]byte)
	newSecret.Data[apicorev1.TLSCertKey] = rootSecret.Data[apicorev1.TLSCertKey]
	newSecret.Data[apicorev1.TLSPrivateKeyKey] = rootSecret.Data[apicorev1.TLSPrivateKeyKey]
	newSecret.Data[gatewaysecret.CACrt] = rootSecret.Data[gatewaysecret.CACrt]

	setCaBundleTimeAnnotationToNow(newSecret)

	return h.client.CreateGatewaySecret(ctx, newSecret)
}

func (h *Handler) requiresCertSwitching(caCertNotBefore time.Time) bool {
	// If the grace period after CA rotation has expired, then we need to switch the certificate and private key
	return time.Now().After(caCertNotBefore.Add(h.serverCertSwitchGracePeriod))
}

func setCaBundleTimeAnnotationToNow(secret *apicorev1.Secret) {
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[shared.CaAddedToBundleAtAnnotation] = apimetav1.Now().Format(time.RFC3339)
	//nolint:godox // valid TODO
	//TODO: drop in the second release after 1.14.0, issue: 3105.
	delete(secret.Annotations, shared.LastModifiedAtAnnotation)
}

func (h *Handler) bundleCACerts(gatewaySecret *apicorev1.Secret, rootSecret *apicorev1.Secret) (bool, error) {
	caBundle := gatewaySecret.Data[gatewaysecret.CACrt]
	cert := rootSecret.Data[apicorev1.TLSCertKey] // tls.crt and ca.crt are the same in the root secret

	bundled, err := h.bundler.Bundle(&caBundle, cert)
	if err != nil {
		return false, fmt.Errorf("failed to bundle root secret's tls.crt into gateway secret's ca.crt: %w", err)
	}

	if !bundled {
		return false, nil
	}

	gatewaySecret.Data[gatewaysecret.CACrt] = caBundle
	return true, nil
}

func (h *Handler) dropExpiredCertsFromBundle(gatewaySecret *apicorev1.Secret) error {
	caBundle := gatewaySecret.Data[gatewaysecret.CACrt]
	certsDropped, err := h.bundler.DropExpiredCerts(&caBundle)
	if err != nil {
		return fmt.Errorf("failed to drop expired certs from gateway secret's ca.crt: %w", err)
	}

	if certsDropped {
		gatewaySecret.Data[gatewaysecret.CACrt] = caBundle
	}

	return nil
}

func (h *Handler) updateServerCertExpiryMetric(gwSecret *apicorev1.Secret) error {
	isCloseToExpiry, err := serverCertCloseToExpiry(gwSecret, h.serverCertExpiryWindow)
	if err != nil {
		return err
	}

	h.metrics.ServerCertificateCloseToExpiry(isCloseToExpiry)
	return nil
}

func serverCertCloseToExpiry(gatewaySecret *apicorev1.Secret, expiryWindow time.Duration) (bool, error) {
	serverCertBytes := gatewaySecret.Data[apicorev1.TLSCertKey]
	block, _ := pem.Decode(serverCertBytes)
	if block == nil {
		return false, ErrServerCertificateParsingFailure
	}
	serverCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrServerCertificateParsingFailure, err)
	}
	if time.Now().Add(expiryWindow).After(serverCert.NotAfter) {
		return true, nil
	}
	return false, nil
}

func switchCertificate(gatewaySecret *apicorev1.Secret, rootSecret *apicorev1.Secret) {
	gatewaySecret.Data[apicorev1.TLSCertKey] = rootSecret.Data[apicorev1.TLSCertKey]
	gatewaySecret.Data[apicorev1.TLSPrivateKeyKey] = rootSecret.Data[apicorev1.TLSPrivateKeyKey]
}
