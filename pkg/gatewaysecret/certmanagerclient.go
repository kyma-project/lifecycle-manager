package gatewaysecret

import (
	"context"

	"k8s.io/client-go/rest"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmanagerclientset "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CertManagerClient struct {
	clientset certmanagerclientset.Clientset
}

func NewCertManagerClient(config *rest.Config) *CertManagerClient {
	return &CertManagerClient{clientset: *certmanagerclientset.NewForConfigOrDie(config)}
}

func (h *CertManagerClient) GetRootCACertificate(ctx context.Context) (*certmanagerv1.Certificate, error) {
	return h.clientset.CertmanagerV1().Certificates(istioNamespace).Get(ctx, kcpCACertName, apimetav1.GetOptions{})
}

func RequiresUpdate(gwSecret *apicorev1.Secret, caCert *certmanagerv1.Certificate) bool {
	if gwSecretLastModifiedAt, err := GetValidLastModifiedAt(gwSecret); err == nil {
		if caCert.Status.NotBefore != nil && gwSecretLastModifiedAt.After(caCert.Status.NotBefore.Time) {
			return false
		}
	}
	return true
}
