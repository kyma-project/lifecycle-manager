package security_test

import (
	"context"
	"crypto/x509"
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/go-logr/zapr"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/kyma-project/lifecycle-manager/pkg/security"
	"github.com/kyma-project/runtime-watcher/listener/pkg/types"
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
					URIs: []*url.URL{{
						Path: "example.domain.com",
					}},
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
					URIs: []*url.URL{{
						Path: "wrong.domain.com",
					}},
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
			require.Empty(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

var _ = Describe("Verify Request using SAN", Ordered, func() {
	zapLog, err := zap.NewDevelopment()
	Expect(err).ShouldNot(HaveOccurred())

	type args struct {
		request            *http.Request
		watcherEventObject *types.WatchEvent
	}

	tests := []struct {
		name    string
		kyma    *v1beta2.Kyma
		args    args
		wantErr bool
	}{
		{
			name: "Verify Request with SAN (Subject Alternative Name)",
			kyma: createKyma("kyma-1", annotationsWithCorrectDomain),
			args: args{
				request:            createRequest("kyma-1", headerWithSufficientCertificate),
				watcherEventObject: createWatcherCR("kyma-1"),
			},
			wantErr: false,
		},
		{
			name: "SKR-Domain Annotation missing on KymaCR",
			kyma: createKyma("kyma-2", emptyAnnotations),
			args: args{
				request:            createRequest("kyma-2", headerWithSufficientCertificate),
				watcherEventObject: createWatcherCR("kyma-2"),
			},
			wantErr: true,
		},
		{
			name: "Malformed Certificate",
			kyma: createKyma("kyma-3", annotationsWithCorrectDomain),
			args: args{
				request:            createRequest("kyma-3", headerWithMalformedCertificate),
				watcherEventObject: createWatcherCR("kyma-3"),
			},
			wantErr: true,
		},
		{
			name: "SAN does not match KymaCR.annotation..skr-domain",
			kyma: createKyma("kyma-4", annotationsWithWrongDomain),
			args: args{
				request:            createRequest("kyma-4", headerWithSufficientCertificate),
				watcherEventObject: createWatcherCR("kyma-4"),
			},
			wantErr: true,
		},
		{
			name: "KymaCR does not exists",
			kyma: nil,
			args: args{
				request:            createRequest("kyma-5", headerWithSufficientCertificate),
				watcherEventObject: createWatcherCR("kyma-5"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		test := tt
		It(test.name, func() {
			// Create Request Verifier
			verifier := &security.RequestVerifier{
				Client: k8sClient,
				Log:    zapr.NewLogger(zapLog),
			}

			// Create Kyma CR
			if test.kyma != nil {
				Expect(k8sClient.Create(context.TODO(), test.kyma)).Should(Succeed())
			}

			// Actual Test
			err := verifier.Verify(test.args.request, test.args.watcherEventObject)
			if test.wantErr {
				Expect(err).Should(HaveOccurred())
				return
			}
			Expect(err).ShouldNot(HaveOccurred())

			// Cleanup
			if test.kyma != nil {
				Expect(k8sClient.Delete(context.TODO(), test.kyma)).Should(Succeed())
			}
		})

	}
},
)
