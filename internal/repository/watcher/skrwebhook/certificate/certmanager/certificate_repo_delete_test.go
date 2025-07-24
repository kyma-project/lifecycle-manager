package certmanager_test

import (
	"context"
	"errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"testing"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/certmanager"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/skrwebhook/certificate"
)

func TestDelete_WhenCalledWithUnknownName_IgnoresNotFoundError(t *testing.T) {
	certRepo := certmanager.NewCertificateRepository(
		newFakeClient(&certmanagerv1.Certificate{}),
		"",
		"some-namespace",
		certificate.CertificateConfig{},
	)

	err := certRepo.Delete(t.Context(), certName)

	require.NoError(t, err)
}

func TestDelete_WhenCalledAndClientReturnsAnyOtherError_ReturnsError(t *testing.T) {
	someError := errors.New("some other error")
	fakeClient := newErrorReturningFakeClient(someError)
	certRepo := certmanager.NewCertificateRepository(
		fakeClient,
		"",
		"",
		certificate.CertificateConfig{},
	)

	err := certRepo.Delete(context.Background(), certName)

	require.ErrorIs(t, err, someError)
}

func TestDelete_WhenCalledWithExistingCertName_ReturnsNoError(t *testing.T) {
	const (
		certName      = "test-cert"
		certNamespace = "test-namespace"
	)
	cert := &certmanagerv1.Certificate{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      certName,
			Namespace: certNamespace,
		},
	}
	certRepo := certmanager.NewCertificateRepository(
		newFakeClient(cert),
		"",
		certNamespace,
		certificate.CertificateConfig{},
	)

	err := certRepo.Delete(context.Background(), certName)

	require.NoError(t, err)
}

func newFakeClient(objs ...runtime.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = certmanagerv1.AddToScheme(scheme)
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		Build()
}

func newErrorReturningFakeClient(err error) client.Client {
	scheme := runtime.NewScheme()
	_ = certmanagerv1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				return err
			},
		}).
		Build()

	return fakeClient
}
