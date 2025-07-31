package data_test

import (
	"testing"

	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate/secret/data"

	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
)

func TestNewGatewaySecretData(t *testing.T) {
	caData := []byte("ca-cert-data")
	tests := []struct {
		name    string
		secret  *apicorev1.Secret
		want    *data.GatewaySecretData
		wantErr bool
	}{
		{
			name:    "nil secret",
			secret:  nil,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "nil data map",
			secret:  &apicorev1.Secret{Data: nil},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "missing ca.crt key",
			secret:  &apicorev1.Secret{Data: map[string][]byte{}},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "ca.crt present",
			secret:  &apicorev1.Secret{Data: map[string][]byte{data.CaCertKey: caData}},
			want:    &data.GatewaySecretData{CaCert: caData},
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := data.NewGatewaySecretData(testCase.secret)
			if testCase.wantErr {
				require.Error(t, err)
				require.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.Equal(t, testCase.want, got)
			}
		})
	}
}

func TestNewCertificateSecretData(t *testing.T) {
	tlsCert := []byte("tls-cert")
	tlsKey := []byte("tls-key")
	tests := []struct {
		name    string
		secret  *apicorev1.Secret
		want    *data.CertificateSecretData
		wantErr bool
	}{
		{
			name:    "nil secret",
			secret:  nil,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "nil data map",
			secret:  &apicorev1.Secret{Data: nil},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "missing tls.crt key",
			secret:  &apicorev1.Secret{Data: map[string][]byte{data.TlsPrivateKeyKey: tlsKey}},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "missing tls.key key",
			secret:  &apicorev1.Secret{Data: map[string][]byte{data.TlsCertKey: tlsCert}},
			want:    nil,
			wantErr: true,
		},
		{
			name: "both keys present",
			secret: &apicorev1.Secret{
				Data: map[string][]byte{
					data.TlsCertKey:       tlsCert,
					data.TlsPrivateKeyKey: tlsKey,
				},
			},
			want:    &data.CertificateSecretData{TlsCert: tlsCert, TlsKey: tlsKey},
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := data.NewCertificateSecretData(testCase.secret)
			if testCase.wantErr {
				require.Error(t, err)
				require.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.Equal(t, testCase.want, got)
			}
		})
	}
}
