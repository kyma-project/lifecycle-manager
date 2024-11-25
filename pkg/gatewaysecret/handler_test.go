package gatewaysecret_test

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/kyma-project/lifecycle-manager/pkg/gatewaysecret"
)

const (
	oldTLSCertValue = "old-value1"
	oldTLSKeyValue  = "old-value2"
	oldCACertValue  = "old-value3"

	newTLSCertValue = "value1"
	newTLSKeyValue  = "value2"
	newCACertValue  = "value3"
)

func TestNewGatewaySecret(t *testing.T) {
	t.Parallel()

	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte(newTLSCertValue),
			"tls.key": []byte(newTLSKeyValue),
			"ca.crt":  []byte(newCACertValue),
		},
	}
	gwSecret := gatewaysecret.NewGatewaySecret(rootSecret)

	require.Equal(t, "klm-istio-gateway", gwSecret.Name)
	require.Equal(t, "istio-system", gwSecret.Namespace)

	require.Equal(t, newTLSCertValue, string(gwSecret.Data["tls.crt"]))
	require.Equal(t, newTLSKeyValue, string(gwSecret.Data["tls.key"]))
	require.Equal(t, newCACertValue, string(gwSecret.Data["ca.crt"]))
}

func TestGetValidLastModifiedAt(t *testing.T) {
	tests := []struct {
		name    string
		secret  *apicorev1.Secret
		want    time.Time
		wantErr bool
	}{
		{
			name: "valid lastModifiedAt annotation",
			secret: &apicorev1.Secret{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{
						"lastModifiedAt": "2024-11-01T00:00:00Z",
					},
				},
			},
			want:    time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name: "missing lastModifiedAt annotation",
			secret: &apicorev1.Secret{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			want:    time.Time{},
			wantErr: true,
		},
		{
			name: "invalid lastModifiedAt annotation key",
			secret: &apicorev1.Secret{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{
						"LastModifiedAt": "2024-11-01T00:00:00Z",
					},
				},
			},
			want:    time.Time{},
			wantErr: true,
		},
		{
			name: "invalid lastModifiedAt annotation time format",
			secret: &apicorev1.Secret{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{
						"lastModifiedAt": "2024-11-01T00:00:00",
					},
				},
			},
			want:    time.Time{},
			wantErr: true,
		},
	}
	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			got, err := gatewaysecret.GetValidLastModifiedAt(testcase.secret)
			if (err != nil) != testcase.wantErr {
				t.Errorf("GetValidLastModifiedAt() error = %v, wantErr %v", err, testcase.wantErr)
				return
			}
			if !reflect.DeepEqual(got, testcase.want) {
				t.Errorf("GetValidLastModifiedAt() got = %v, want %v", got, testcase.want)
			}
		})
	}
}

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
			if got := gatewaysecret.GatewaySecretRequiresUpdate(
				testcase.args.gwSecret, testcase.args.caCert); got != testcase.want {
				t.Errorf("GatewaySecretRequiresUpdate() = %v, want %v", got, testcase.want)
			}
		})
	}
}

func TestCopyRootSecretDataIntoGatewaySecret(t *testing.T) {
	t.Parallel()

	gwSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte(oldTLSCertValue),
			"tls.key": []byte(oldTLSKeyValue),
			"ca.crt":  []byte(oldCACertValue),
		},
	}

	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte(newTLSCertValue),
			"tls.key": []byte(newTLSKeyValue),
			"ca.crt":  []byte(newCACertValue),
		},
	}

	gatewaysecret.CopyRootSecretDataIntoGatewaySecret(gwSecret, rootSecret)

	require.Equal(t, newTLSCertValue, string(gwSecret.Data["tls.crt"]))
	require.Equal(t, newTLSKeyValue, string(gwSecret.Data["tls.key"]))
	require.Equal(t, newCACertValue, string(gwSecret.Data["ca.crt"]))
}

func TestWatchEvents(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	var waitGroup sync.WaitGroup
	waitGroup.Add(1)

	calledTimes := 0
	//nolint:unparam The function has to have this signature to be used in WatchEvents
	mockManageSecretFunc := func(_ context.Context, _ *apicorev1.Secret) error {
		calledTimes += 1
		return nil
	}

	events := make(chan watch.Event, 1)
	go func() {
		defer waitGroup.Done()
		gatewaysecret.WatchEvents(ctx, events, mockManageSecretFunc, logr.Logger{})
	}()

	events <- watch.Event{
		Type: watch.Added,
		Object: &apicorev1.Secret{
			Data: map[string][]byte{
				"tls.crt": []byte(newTLSCertValue),
				"tls.key": []byte(newTLSKeyValue),
				"ca.crt":  []byte(newCACertValue),
			},
		},
	}
	events <- watch.Event{
		Type: watch.Modified,
		Object: &apicorev1.Secret{
			Data: map[string][]byte{
				"tls.crt": []byte(newTLSCertValue),
				"tls.key": []byte(newTLSKeyValue),
				"ca.crt":  []byte(newCACertValue),
			},
		},
	}
	close(events)
	waitGroup.Wait()

	require.Equal(t, 2, calledTimes)
}
