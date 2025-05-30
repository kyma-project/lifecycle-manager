package resources

import (
	"k8s.io/api/core/v1"
	v2 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate/secret"
)

const (
	SkrTLSName = "skr-webhook-tls"
)

func BuildSKRSecret(caCert, tlsCert, tlsKey []byte, remoteNs string) *v1.Secret {
	return &v1.Secret{
		TypeMeta: v2.TypeMeta{
			Kind:       "Secret",
			APIVersion: v1.SchemeGroupVersion.String(),
		},
		ObjectMeta: v2.ObjectMeta{
			Name:      SkrTLSName,
			Namespace: remoteNs,
			Labels: map[string]string{
				shared.ManagedBy: shared.ManagedByLabelValue,
			},
		},
		Immutable: nil,
		Data: map[string][]byte{
			secret.CaCertKey:        caCert,
			secret.TlsCertKey:       tlsCert,
			secret.TlsPrivateKeyKey: tlsKey,
		},
		Type: v1.SecretTypeOpaque,
	}
}
