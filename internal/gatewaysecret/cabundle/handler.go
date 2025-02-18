package cabundle

import (
	"context"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/gatewaysecret"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

const (
	caBundleTempCertKey           = "temp.ca.crt"
	currentCAExpirationAnnotation = "currentCAExpiration"
)

type Handler struct {
	client                         gatewaysecret.Client
	parseLastModifiedTime          gatewaysecret.TimeParserFunc
	switchCertBeforeExpirationTime time.Duration
}

func NewGatewaySecretHandler(client gatewaysecret.Client, timeParserFunc gatewaysecret.TimeParserFunc,
	switchCertBeforeExpirationTime time.Duration,
) *Handler {
	return &Handler{
		client:                         client,
		parseLastModifiedTime:          timeParserFunc,
		switchCertBeforeExpirationTime: switchCertBeforeExpirationTime,
	}
}

func (h *Handler) ManageGatewaySecret(ctx context.Context, rootSecret *apicorev1.Secret) error {
	caCert, err := h.client.GetWatcherServingCert(ctx)
	if err != nil {
		return err
	}

	gwSecret, err := h.client.GetGatewaySecret(ctx)
	if util.IsNotFound(err) {
		return h.createGatewaySecretFromRootSecret(ctx, rootSecret, caCert)
	} else if err != nil {
		return err
	}

	if h.requiresBundling(gwSecret, caCert) {
		bundleCACrt(gwSecret, rootSecret)
		setLastModifiedToNow(gwSecret)
	}
	if h.requiresCertSwitching(gwSecret) {
		switchCertificate(gwSecret, rootSecret)
		setCurrentCAExpiration(gwSecret, caCert)
	}
	return h.client.UpdateGatewaySecret(ctx, gwSecret)
}

func (h *Handler) createGatewaySecretFromRootSecret(ctx context.Context, rootSecret *apicorev1.Secret,
	caCert *certmanagerv1.Certificate,
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
	setCurrentCAExpiration(newSecret, caCert)

	return h.client.CreateGatewaySecret(ctx, newSecret)
}

func (h *Handler) requiresBundling(gwSecret *apicorev1.Secret, caCert *certmanagerv1.Certificate) bool {
	// If the last modified time of the gateway secret is after the notBefore time of the CA certificate,
	// then we don't need to update the gateway secret
	if lastModified, err := h.parseLastModifiedTime(gwSecret, shared.LastModifiedAtAnnotation); err == nil {
		if caCert.Status.NotBefore != nil && lastModified.After(caCert.Status.NotBefore.Time) {
			return false
		}
	}
	return true
}

func (h *Handler) requiresCertSwitching(gwSecret *apicorev1.Secret) bool {
	// If the current CA is about to expire, then we need to switch the certificate and private key
	caExpirationTime, err := h.parseLastModifiedTime(gwSecret, currentCAExpirationAnnotation)
	return err != nil || time.Now().After(caExpirationTime.Add(-h.switchCertBeforeExpirationTime))
}

func setLastModifiedToNow(secret *apicorev1.Secret) {
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[shared.LastModifiedAtAnnotation] = apimetav1.Now().Format(time.RFC3339)
}

func setCurrentCAExpiration(secret *apicorev1.Secret, caCert *certmanagerv1.Certificate) {
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	if caCert.Status.NotAfter != nil {
		secret.Annotations[currentCAExpirationAnnotation] = caCert.Status.NotAfter.Time.Format(time.RFC3339)
	}
}

func bundleCACrt(gatewaySecret *apicorev1.Secret, rootSecret *apicorev1.Secret) {
	//nolint:gocritic // we need to append the new CA cert to the existing CA cert
	gatewaySecret.Data[gatewaysecret.CACrt] = append(rootSecret.Data[gatewaysecret.CACrt],
		gatewaySecret.Data[caBundleTempCertKey]...)
	gatewaySecret.Data[caBundleTempCertKey] = rootSecret.Data[gatewaysecret.CACrt]
}

func switchCertificate(gatewaySecret *apicorev1.Secret, rootSecret *apicorev1.Secret) {
	gatewaySecret.Data[gatewaysecret.TLSCrt] = rootSecret.Data[gatewaysecret.TLSCrt]
	gatewaySecret.Data[gatewaysecret.TLSKey] = rootSecret.Data[gatewaysecret.TLSKey]
}
