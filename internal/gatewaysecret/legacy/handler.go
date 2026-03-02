package legacy

import (
	"context"
	"time"

	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/gatewaysecret"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type Handler struct {
	client                gatewaysecret.Client
	parseLastModifiedTime gatewaysecret.TimeParserFunc
}

func NewGatewaySecretHandler(client gatewaysecret.Client, timeParserFunc gatewaysecret.TimeParserFunc) *Handler {
	return &Handler{
		client:                client,
		parseLastModifiedTime: timeParserFunc,
	}
}

func (h *Handler) ManageGatewaySecret(ctx context.Context, rootSecret *apicorev1.Secret) error {
	gwSecret, err := h.client.GetGatewaySecret(ctx)
	if util.IsNotFound(err) {
		return h.createGatewaySecretFromRootSecret(ctx, rootSecret)
	} else if err != nil {
		return err
	}

	notBefore, _, err := h.client.GetWatcherServingCertValidity(ctx)
	if err != nil {
		return err
	}

	if h.requiresUpdate(gwSecret, notBefore) {
		copyDataFromRootSecret(gwSecret, rootSecret)
		setCaBundleTimeAnnotationToNow(gwSecret)

		return h.client.UpdateGatewaySecret(ctx, gwSecret)
	}

	return nil
}

func (h *Handler) createGatewaySecretFromRootSecret(ctx context.Context, rootSecret *apicorev1.Secret) error {
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

	copyDataFromRootSecret(newSecret, rootSecret)
	setCaBundleTimeAnnotationToNow(newSecret)

	return h.client.CreateGatewaySecret(ctx, newSecret)
}

func (h *Handler) requiresUpdate(gwSecret *apicorev1.Secret, notBefore time.Time) bool {
	// If the last modified time of the gateway secret is after the notBefore time of the CA certificate,
	// then we don't need to update the gateway secret
	if caBundleExtendedAt, err := h.parseLastModifiedTime(gwSecret, shared.CaAddedToBundleAtAnnotation); err == nil {
		if !notBefore.IsZero() && caBundleExtendedAt.After(notBefore) {
			return false
		}
	}
	return true
}

func setCaBundleTimeAnnotationToNow(secret *apicorev1.Secret) {
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[shared.CaAddedToBundleAtAnnotation] = apimetav1.Now().Format(time.RFC3339)
	delete(secret.Annotations, shared.LastModifiedAtAnnotation)
}

func copyDataFromRootSecret(secret *apicorev1.Secret, rootSecret *apicorev1.Secret) {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[apicorev1.TLSCertKey] = rootSecret.Data[apicorev1.TLSCertKey]
	secret.Data[apicorev1.TLSPrivateKeyKey] = rootSecret.Data[apicorev1.TLSPrivateKeyKey]
	secret.Data[gatewaysecret.CACrt] = rootSecret.Data[gatewaysecret.CACrt]
}
