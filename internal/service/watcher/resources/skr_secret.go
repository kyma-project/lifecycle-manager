package resources

import (
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate/secret/data"
)

const (
	SkrTLSName = "skr-webhook-tls"
)

func BuildSKRSecret(caCert, tlsCert, tlsKey []byte, remoteNs string) *apicorev1.Secret {
	return &apicorev1.Secret{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: apicorev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      SkrTLSName,
			Namespace: remoteNs,
			Labels: map[string]string{
				shared.ManagedBy: shared.ManagedByLabelValue,
			},
		},
		Immutable: nil,
		Data: map[string][]byte{
			data.CaCertKey:             caCert,
			apicorev1.TLSCertKey:       tlsCert,
			apicorev1.TLSPrivateKeyKey: tlsKey,
		},
	}
}
