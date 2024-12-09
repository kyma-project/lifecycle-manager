package gatewaysecret_test

import (
	"testing"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/pkg/gatewaysecret"
)

func TestGatewaySecretRequiresUpdate(t *testing.T) {
	type args struct {
		gwSecret *apicorev1.Secret
		caCert   certmanagerv1.Certificate
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "gateway secret is newer than CA certificate",
			args: args{
				gwSecret: &apicorev1.Secret{
					ObjectMeta: apimetav1.ObjectMeta{
						Annotations: map[string]string{
							"lastModifiedAt": "2024-11-01T00:00:10Z",
						},
					},
				},
				caCert: certmanagerv1.Certificate{
					Status: certmanagerv1.CertificateStatus{
						NotBefore: &apimetav1.Time{
							Time: time.Date(2024, 11, 1, 0, 0, 5, 0, time.UTC),
						},
					},
				},
			},
			want: false,
		},
		{
			name: "gateway secret is older than CA certificate",
			args: args{
				gwSecret: &apicorev1.Secret{
					ObjectMeta: apimetav1.ObjectMeta{
						Annotations: map[string]string{
							"lastModifiedAt": "2024-11-01T00:00:00Z",
						},
					},
				},
				caCert: certmanagerv1.Certificate{
					Status: certmanagerv1.CertificateStatus{
						NotBefore: &apimetav1.Time{
							Time: time.Date(2024, 11, 1, 0, 0, 5, 0, time.UTC),
						},
					},
				},
			},
			want: true,
		},
		{
			name: "gateway secret has parsing error for lastModifiedAt",
			args: args{
				gwSecret: &apicorev1.Secret{
					ObjectMeta: apimetav1.ObjectMeta{
						Annotations: map[string]string{
							"lastModifiedAt": "2024-11-01T00:00:00",
						},
					},
				},
				caCert: certmanagerv1.Certificate{
					Status: certmanagerv1.CertificateStatus{
						NotBefore: &apimetav1.Time{
							Time: time.Date(2024, 11, 1, 0, 0, 5, 0, time.UTC),
						},
					},
				},
			},
			want: true,
		},
		{
			name: "gateway secret is missing lastModifiedAt",
			args: args{
				gwSecret: &apicorev1.Secret{
					ObjectMeta: apimetav1.ObjectMeta{
						Annotations: map[string]string{},
					},
				},
				caCert: certmanagerv1.Certificate{
					Status: certmanagerv1.CertificateStatus{
						NotBefore: &apimetav1.Time{
							Time: time.Date(2024, 11, 1, 0, 0, 5, 0, time.UTC),
						},
					},
				},
			},
			want: true,
		},
	}
	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			if got := gatewaysecret.RequiresUpdate(
				testcase.args.gwSecret, &testcase.args.caCert); got != testcase.want {
				t.Errorf("RequiresUpdate() = %v, want %v", got, testcase.want)
			}
		})
	}
}

func TestCopyRootSecretDataIntoGatewaySecret(t *testing.T) {
	t.Parallel()

	gwSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte(("old-value1")),
			"tls.key": []byte(("old-value2")),
			"ca.crt":  []byte(("old-value3")),
		},
	}

	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte(("value1")),
			"tls.key": []byte(("value2")),
			"ca.crt":  []byte(("value3")),
		},
	}

	gatewaysecret.CopySecretData(rootSecret, gwSecret)

	require.Equal(t, "value1", string(gwSecret.Data["tls.crt"]))
	require.Equal(t, "value2", string(gwSecret.Data["tls.key"]))
	require.Equal(t, "value3", string(gwSecret.Data["ca.crt"]))
}
