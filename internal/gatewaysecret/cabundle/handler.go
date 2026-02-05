package cabundle

import (
	"context"
	"errors"
	"fmt"
	"time"

	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/gatewaysecret"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var ErrCACertificateNotReady = errors.New("watcher-serving ca certificate is not ready")

const (
	caBundleTempCertKey           = "temp.ca.crt"
	CurrentCAExpirationAnnotation = "currentCAExpiration"
)

type Bundler interface {
	Bundle(bundle *[]byte, cert []byte) (bool, error)
	DropExpiredCerts(bundle *[]byte) (bool, error)
}

type Handler struct {
	client                         gatewaysecret.Client
	parseTimeFromAnnotationFunc    gatewaysecret.TimeParserFunc
	switchCertBeforeExpirationTime time.Duration
	bundler                        Bundler
}

func NewGatewaySecretHandler(client gatewaysecret.Client,
	timeParserFunc gatewaysecret.TimeParserFunc,
	switchCertBeforeExpirationTime time.Duration,
	bundler Bundler,
) *Handler {
	return &Handler{
		client:                         client,
		parseTimeFromAnnotationFunc:    timeParserFunc,
		switchCertBeforeExpirationTime: switchCertBeforeExpirationTime,
		bundler:                        bundler,
	}
}

func (h *Handler) ManageGatewaySecret(ctx context.Context, rootSecret *apicorev1.Secret) error {
	notBefore, notAfter, err := h.client.GetWatcherServingCertValidity(ctx)
	if err != nil {
		return err
	}
	if notBefore.IsZero() || notAfter.IsZero() {
		return ErrCACertificateNotReady
	}

	gwSecret, err := h.client.GetGatewaySecret(ctx)
	if util.IsNotFound(err) {
		return h.createGatewaySecretFromRootSecret(ctx, rootSecret, notAfter)
	} else if err != nil {
		return err
	}

	// this is for the case when we switch existing secret from legacy to new rotation mechanism
	bootstrapLegacyGatewaySecret(gwSecret, rootSecret, notAfter)

	bundled, err := h.bundleCACerts(gwSecret, rootSecret)
	if err != nil {
		return err
	}

	if bundled {
		setLastModifiedToNow(gwSecret)
	}

	err = h.dropExpiredCertsFromBundle(gwSecret)
	if err != nil {
		return err
	}

	if h.requiresCertSwitching(gwSecret) {
		switchCertificate(gwSecret, rootSecret)
		setCurrentCAExpiration(gwSecret, notAfter)
	}
	return h.client.UpdateGatewaySecret(ctx, gwSecret)
}

func (h *Handler) createGatewaySecretFromRootSecret(ctx context.Context,
	rootSecret *apicorev1.Secret,
	notAfter time.Time,
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
	newSecret.Data[gatewaysecret.TLSCrt] = rootSecret.Data[gatewaysecret.TLSCrt]
	newSecret.Data[gatewaysecret.TLSKey] = rootSecret.Data[gatewaysecret.TLSKey]
	newSecret.Data[gatewaysecret.CACrt] = rootSecret.Data[gatewaysecret.CACrt]

	newSecret.Data[caBundleTempCertKey] = rootSecret.Data[gatewaysecret.CACrt]
	setLastModifiedToNow(newSecret)
	setCurrentCAExpiration(newSecret, notAfter)

	return h.client.CreateGatewaySecret(ctx, newSecret)
}

func (h *Handler) requiresCertSwitching(gwSecret *apicorev1.Secret) bool {
	// If the current CA is about to expire, then we need to switch the certificate and private key
	caExpirationTime, err := h.parseTimeFromAnnotationFunc(gwSecret, CurrentCAExpirationAnnotation)
	return err != nil || time.Now().After(caExpirationTime.Add(-h.switchCertBeforeExpirationTime))
}

func bootstrapLegacyGatewaySecret(gwSecret *apicorev1.Secret,
	rootSecret *apicorev1.Secret,
	notAfter time.Time,
) {
	if _, ok := gwSecret.Annotations[CurrentCAExpirationAnnotation]; !ok {
		setCurrentCAExpiration(gwSecret, notAfter)
	}
	if _, ok := gwSecret.Data[caBundleTempCertKey]; !ok {
		gwSecret.Data[caBundleTempCertKey] = rootSecret.Data[gatewaysecret.CACrt]
	}
}

func setLastModifiedToNow(secret *apicorev1.Secret) {
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[shared.LastModifiedAtAnnotation] = apimetav1.Now().Format(time.RFC3339)
}

func setCurrentCAExpiration(secret *apicorev1.Secret, notAfter time.Time) {
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[CurrentCAExpirationAnnotation] = notAfter.Format(time.RFC3339)
}

func (h *Handler) bundleCACerts(gatewaySecret *apicorev1.Secret, rootSecret *apicorev1.Secret) (bool, error) {
	caBundle := gatewaySecret.Data[gatewaysecret.CACrt]
	cert := rootSecret.Data[gatewaysecret.TLSCrt] // tls.crt and ca.crt are the same in the root secret

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

func switchCertificate(gatewaySecret *apicorev1.Secret, rootSecret *apicorev1.Secret) {
	gatewaySecret.Data[gatewaysecret.TLSCrt] = rootSecret.Data[gatewaysecret.TLSCrt]
	gatewaySecret.Data[gatewaysecret.TLSKey] = rootSecret.Data[gatewaysecret.TLSKey]
}
