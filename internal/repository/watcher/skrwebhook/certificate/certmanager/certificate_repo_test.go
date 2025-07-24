package certmanager_test

import (
	"context"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/certmanager"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/skrwebhook/certificate"
	"k8s.io/apimachinery/pkg/api/meta"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"testing"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

var (
	issuerName         = random.Name()
	certName           = random.Name()
	certNamespace      = random.Name()
	certCommonNameName = random.Name()
	certDNSNames       = []string{
		random.Name(),
		random.Name(),
	}
	certDuration    = 24 * time.Hour
	certRenewBefore = 12 * time.Hour
	certKeySize     = 4096
)

func Test_GetCacheObjects(t *testing.T) {
	objects := certmanager.GetCacheObjects()
	require.Len(t, objects, 1)
	assert.IsType(t, &certmanagerv1.Certificate{}, objects[0])
}

func Test_CertificateClient_GetRenewalTime_Success(t *testing.T) {
	clientStub := &kcpClientStub{
		getCert: &certmanagerv1.Certificate{
			TypeMeta: apimetav1.TypeMeta{
				Kind:       certmanagerv1.CertificateKind,
				APIVersion: certmanagerv1.SchemeGroupVersion.String(),
			},
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      certName,
				Namespace: certNamespace,
			},
			Status: certmanagerv1.CertificateStatus{
				RenewalTime: &apimetav1.Time{Time: time.Now().Add(-time.Hour)},
			},
		},
	}
	certClient := certmanager.NewCertificateRepository(
		clientStub,
		issuerName,
		certNamespace,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	time, err := certClient.GetRenewalTime(t.Context(), certName)

	require.NoError(t, err)
	assert.Equal(t, clientStub.getCert.Status.RenewalTime.Time, time)
	assert.True(t, clientStub.getCalled)
}

func Test_CertificateClient_GetRenewalTime_Error(t *testing.T) {
	clientStub := &kcpClientStub{
		getErr: assert.AnError,
	}
	certClient := certmanager.NewCertificateRepository(
		clientStub,
		issuerName,
		certNamespace,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	time, err := certClient.GetRenewalTime(t.Context(), certName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get certificate")
	assert.True(t, time.IsZero())
	assert.True(t, clientStub.getCalled)
}

func Test_CertificateClient_GetRenewalTime_NoRenewalTime(t *testing.T) {
	clientStub := &kcpClientStub{
		getCert: &certmanagerv1.Certificate{
			TypeMeta: apimetav1.TypeMeta{
				Kind:       certmanagerv1.CertificateKind,
				APIVersion: certmanagerv1.SchemeGroupVersion.String(),
			},
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      certName,
				Namespace: certNamespace,
			},
			Status: certmanagerv1.CertificateStatus{},
		},
	}
	certClient := certmanager.NewCertificateRepository(
		clientStub,
		issuerName,
		certNamespace,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	time, err := certClient.GetRenewalTime(t.Context(), certName)

	require.Error(t, err)
	assert.Equal(t, certificate.ErrNoRenewalTime, err)
	assert.True(t, time.IsZero())
	assert.True(t, clientStub.getCalled)
}

func Test_CertificateClient_GetValidity_Success(t *testing.T) {
	clientStub := &kcpClientStub{
		getCert: &certmanagerv1.Certificate{
			TypeMeta: apimetav1.TypeMeta{
				Kind:       certmanagerv1.CertificateKind,
				APIVersion: certmanagerv1.SchemeGroupVersion.String(),
			},
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      certName,
				Namespace: certNamespace,
			},
			Status: certmanagerv1.CertificateStatus{
				NotBefore: &apimetav1.Time{Time: time.Now().Add(-time.Hour)},
				NotAfter:  &apimetav1.Time{Time: time.Now().Add(time.Hour)},
			},
		},
	}
	certClient := certmanager.NewCertificateRepository(
		clientStub,
		issuerName,
		certNamespace,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	notBefore, notAfter, err := certClient.GetValidity(t.Context(), certName, certNamespace)

	require.NoError(t, err)
	assert.Equal(t, clientStub.getCert.Status.NotBefore.Time, notBefore)
	assert.Equal(t, clientStub.getCert.Status.NotAfter.Time, notAfter)
	assert.True(t, clientStub.getCalled)
}

func Test_CertificateClient_GetValidity_NoNotBefore(t *testing.T) {
	clientStub := &kcpClientStub{
		getCert: &certmanagerv1.Certificate{
			TypeMeta: apimetav1.TypeMeta{
				Kind:       certmanagerv1.CertificateKind,
				APIVersion: certmanagerv1.SchemeGroupVersion.String(),
			},
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      certName,
				Namespace: certNamespace,
			},
			Status: certmanagerv1.CertificateStatus{
				NotAfter: &apimetav1.Time{Time: time.Now().Add(time.Hour)},
			},
		},
	}
	certClient := certmanager.NewCertificateRepository(
		clientStub,
		issuerName,
		certNamespace,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	notBefore, notAfter, err := certClient.GetValidity(t.Context(), certName, certNamespace)

	require.Error(t, err)
	assert.Equal(t, certmanager.ErrNoNotBefore, err)
	assert.True(t, notBefore.IsZero())
	assert.True(t, notAfter.IsZero())
	assert.True(t, clientStub.getCalled)
}

func Test_CertificateClient_GetValidity_NoNotAfter(t *testing.T) {
	clientStub := &kcpClientStub{
		getCert: &certmanagerv1.Certificate{
			TypeMeta: apimetav1.TypeMeta{
				Kind:       certmanagerv1.CertificateKind,
				APIVersion: certmanagerv1.SchemeGroupVersion.String(),
			},
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      certName,
				Namespace: certNamespace,
			},
			Status: certmanagerv1.CertificateStatus{
				NotBefore: &apimetav1.Time{Time: time.Now().Add(-time.Hour)},
			},
		},
	}
	certClient := certmanager.NewCertificateRepository(
		clientStub,
		issuerName,
		certNamespace,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	notBefore, notAfter, err := certClient.GetValidity(t.Context(), certName, certNamespace)

	require.Error(t, err)
	assert.Equal(t, certmanager.ErrNoNotAfter, err)
	assert.True(t, notBefore.IsZero())
	assert.True(t, notAfter.IsZero())
	assert.True(t, clientStub.getCalled)
}

func Test_CertificateClient_GetValidity_GetError(t *testing.T) {
	clientStub := &kcpClientStub{
		getErr: assert.AnError,
	}
	certClient := certmanager.NewCertificateRepository(
		clientStub,
		issuerName,
		certNamespace,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	notBefore, notAfter, err := certClient.GetValidity(t.Context(), certName, certNamespace)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get certificate")
	assert.Zero(t, notBefore)
	assert.Zero(t, notAfter)
	assert.True(t, clientStub.getCalled)
}

// Test stubs

type kcpClientStub struct {
	getCert      *certmanagerv1.Certificate
	getCalled    bool
	getErr       error
	deleteCalled bool
	deleteErr    error
	deleteArg    *certmanagerv1.Certificate
	patchCalled  bool
	patchErr     error
	patchArg     *certmanagerv1.Certificate
}

func (c *kcpClientStub) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	//TODO implement me
	panic("implement me")
}

func (c *kcpClientStub) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	//TODO implement me
	panic("implement me")
}

func (c *kcpClientStub) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	//TODO implement me
	panic("implement me")
}

func (c *kcpClientStub) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	//TODO implement me
	panic("implement me")
}

func (c *kcpClientStub) Status() client.SubResourceWriter {
	//TODO implement me
	panic("implement me")
}

func (c *kcpClientStub) SubResource(subResource string) client.SubResourceClient {
	//TODO implement me
	panic("implement me")
}

func (c *kcpClientStub) Scheme() *machineryruntime.Scheme {
	//TODO implement me
	panic("implement me")
}

func (c *kcpClientStub) RESTMapper() meta.RESTMapper {
	//TODO implement me
	panic("implement me")
}

func (c *kcpClientStub) GroupVersionKindFor(obj machineryruntime.Object) (schema.GroupVersionKind, error) {
	//TODO implement me
	panic("implement me")
}

func (c *kcpClientStub) IsObjectNamespaced(obj machineryruntime.Object) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (c *kcpClientStub) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	c.getCalled = true
	if c.getCert != nil {
		c.getCert.DeepCopyInto(obj.(*certmanagerv1.Certificate))
	}
	return c.getErr
}

func (c *kcpClientStub) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	c.deleteCalled = true
	c.deleteArg = obj.(*certmanagerv1.Certificate)
	return c.deleteErr
}

func (c *kcpClientStub) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	c.patchCalled = true
	c.patchArg = obj.(*certmanagerv1.Certificate)
	return c.patchErr
}
