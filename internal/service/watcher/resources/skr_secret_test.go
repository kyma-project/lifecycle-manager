package resources_test

import (
	"reflect"
	"testing"

	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate/secret/data"
	skrwebhookresources "github.com/kyma-project/lifecycle-manager/internal/service/watcher/resources"
)

func TestBuildSKRSecret(t *testing.T) {
	type args struct {
		caCert   []byte
		tlsCert  []byte
		tlsKey   []byte
		remoteNs string
	}
	tests := []struct {
		name string
		args args
		want *apicorev1.Secret
	}{
		{
			name: "builds secret with correct fields",
			args: args{
				caCert:   []byte("ca"),
				tlsCert:  []byte("cert"),
				tlsKey:   []byte("key"),
				remoteNs: "test-ns",
			},
			want: &apicorev1.Secret{
				TypeMeta: apimetav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: apicorev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      skrwebhookresources.SkrTLSName,
					Namespace: "test-ns",
					Labels: map[string]string{
						shared.ManagedBy: shared.ManagedByLabelValue,
					},
				},
				Immutable: nil,
				Data: map[string][]byte{
					data.CaCertKey:        []byte("ca"),
					data.TlsCertKey:       []byte("cert"),
					data.TlsPrivateKeyKey: []byte("key"),
				},
				Type: apicorev1.SecretTypeOpaque,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := skrwebhookresources.BuildSKRSecret(tt.args.caCert, tt.args.tlsCert, tt.args.tlsKey,
				tt.args.remoteNs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BuildSKRSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}
