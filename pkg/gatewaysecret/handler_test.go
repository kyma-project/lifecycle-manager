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
	"k8s.io/apimachinery/pkg/api/meta"
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
			name: "gateway secret is newer but has parsing error for lastModifiedAt",
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

	// Current gateway secret
	gwSecret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Annotations: map[string]string{
				"lastModifiedAt": "2024-11-01T00:00:00Z",
			},
		},
		Data: map[string][]byte{
			"tls.crt": []byte(oldTLSCertValue),
			"tls.key": []byte(oldTLSKeyValue),
			"ca.crt":  []byte(oldCACertValue),
		},
	}

	// Newer than gateway secret
	caCert := certmanagerv1.Certificate{
		Status: certmanagerv1.CertificateStatus{
			NotBefore: &apimetav1.Time{
				Time: time.Date(2024, 11, 1, 0, 0, 5, 0, time.UTC),
			},
		},
	}
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte(newTLSCertValue),
			"tls.key": []byte(newTLSKeyValue),
			"ca.crt":  []byte(newCACertValue),
		},
	}

	gatewaysecret.CopyRootSecretDataIntoGatewaySecret(gwSecret, caCert, rootSecret)

	require.Equal(t, string(gwSecret.Data["tls.crt"]), newTLSCertValue)
	require.Equal(t, string(gwSecret.Data["tls.key"]), newTLSKeyValue)
	require.Equal(t, string(gwSecret.Data["ca.crt"]), newCACertValue)
}

type MockSecretManager struct {
	findGatewaySecretFunc    func(ctx context.Context) (*apicorev1.Secret, error)
	getRootCACertificateFunc func(ctx context.Context) (certmanagerv1.Certificate, error)
	createFunc               func(ctx context.Context, secret *apicorev1.Secret) error
	updateFunc               func(ctx context.Context, secret *apicorev1.Secret) error
}

func (m MockSecretManager) FindGatewaySecret(ctx context.Context) (*apicorev1.Secret, error) {
	return m.findGatewaySecretFunc(ctx)
}

func (m MockSecretManager) GetRootCACertificate(ctx context.Context) (certmanagerv1.Certificate, error) {
	return m.getRootCACertificateFunc(ctx)
}

func (m MockSecretManager) Create(ctx context.Context, secret *apicorev1.Secret) error {
	return m.createFunc(ctx, secret)
}

func (m MockSecretManager) Update(ctx context.Context, secret *apicorev1.Secret) error {
	return m.updateFunc(ctx, secret)
}

func TestWatchEventsNewGatewaySecret(t *testing.T) {
	t.Parallel()

	findGatewaySecretFunc := func(ctx context.Context) (*apicorev1.Secret, error) {
		return nil, &meta.NoResourceMatchError{}
	}
	createFunc := func(ctx context.Context, gwSecret *apicorev1.Secret) error {
		require.Equal(t, string(gwSecret.Data["tls.crt"]), newTLSCertValue)
		require.Equal(t, string(gwSecret.Data["tls.key"]), newTLSKeyValue)
		require.Equal(t, string(gwSecret.Data["ca.crt"]), newCACertValue)

		return nil
	}
	mockManager := MockSecretManager{findGatewaySecretFunc: findGatewaySecretFunc, createFunc: createFunc}
	handler := gatewaysecret.GatewaySecretHandler{SecretManager: mockManager}

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	var waitGroup sync.WaitGroup
	waitGroup.Add(1)

	events := make(chan watch.Event, 1)
	go func() {
		defer waitGroup.Done()
		gatewaysecret.WatchEvents(ctx, events, &handler, logr.Logger{})
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
	close(events)

	waitGroup.Wait()
}

func TestWatchEventsExistingGatewaySecret(t *testing.T) {
	t.Parallel()

	findGatewaySecretFunc := func(ctx context.Context) (*apicorev1.Secret, error) {
		return &apicorev1.Secret{
			ObjectMeta: apimetav1.ObjectMeta{
				Annotations: map[string]string{
					"lastModifiedAt": "2024-11-01T00:00:00Z",
				},
			},
			Data: map[string][]byte{
				"tls.crt": []byte(oldTLSCertValue),
				"tls.key": []byte(oldTLSKeyValue),
				"ca.crt":  []byte(oldCACertValue),
			},
		}, nil
	}
	getRootCACertificateFunc := func(ctx context.Context) (certmanagerv1.Certificate, error) {
		return certmanagerv1.Certificate{
			Status: certmanagerv1.CertificateStatus{
				NotBefore: &apimetav1.Time{
					Time: time.Date(2024, 11, 1, 0, 0, 5, 0, time.UTC),
				},
			},
		}, nil
	}
	updateFunc := func(ctx context.Context, gwSecret *apicorev1.Secret) error {
		require.Equal(t, string(gwSecret.Data["tls.crt"]), newTLSCertValue)
		require.Equal(t, string(gwSecret.Data["tls.key"]), newTLSKeyValue)
		require.Equal(t, string(gwSecret.Data["ca.crt"]), newCACertValue)

		return nil
	}

	mockManager := MockSecretManager{
		findGatewaySecretFunc:    findGatewaySecretFunc,
		getRootCACertificateFunc: getRootCACertificateFunc, updateFunc: updateFunc,
	}
	handler := gatewaysecret.GatewaySecretHandler{SecretManager: mockManager}

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	var waitGroupg sync.WaitGroup
	waitGroupg.Add(1)

	events := make(chan watch.Event, 1)
	go func() {
		defer waitGroupg.Done()
		gatewaysecret.WatchEvents(ctx, events, &handler, logr.Logger{})
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
	close(events)

	waitGroupg.Wait()
}
