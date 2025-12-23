package security_test

import (
	"crypto/x509"
	"net"
	"net/url"
	"testing"

	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/kyma-project/lifecycle-manager/pkg/security"
)

func TestRequestVerifier_verifySAN(t *testing.T) {
	t.Parallel()

	type args struct {
		certificate *x509.Certificate
		kymaDomain  string
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Proper subject alternative name - IPs",
			args: args{
				certificate: &x509.Certificate{
					IPAddresses: []net.IP{{127, 0, 0, 1}},
					URIs:        nil,
					DNSNames:    nil,
				},
				kymaDomain: "127.0.0.1",
			},
			want: true,
		},
		{
			name: "Proper subject alternative name - URI",
			args: args{
				certificate: &x509.Certificate{
					IPAddresses: nil,
					URIs: []*url.URL{
						{Path: "example.domain.com"},
					},
					DNSNames: nil,
				},
				kymaDomain: "example.domain.com",
			},
			want: true,
		},
		{
			name: "Proper subject alternative name - DNS",
			args: args{
				certificate: &x509.Certificate{
					IPAddresses: nil,
					URIs:        nil,
					DNSNames:    []string{"example.domain.com"},
				},
				kymaDomain: "example.domain.com",
			},
			want: true,
		},
		{
			name: "Certificate has different SAN",
			args: args{
				certificate: &x509.Certificate{
					IPAddresses: []net.IP{{192, 168, 127, 1}, {192, 168, 127, 2}},
					URIs: []*url.URL{
						{Path: "wrong.domain.com"},
					},
					DNSNames: []string{"wrong.domain.com"},
				},
				kymaDomain: "example.domain.com",
			},
			want: false,
		},
	}

	zapLog, err := zap.NewDevelopment()
	require.NoError(t, err)

	verifier := &security.RequestVerifier{
		Client: nil,
		Log:    zapr.NewLogger(zapLog),
	}

	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := verifier.VerifySAN(test.args.certificate, test.args.kymaDomain)
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}
