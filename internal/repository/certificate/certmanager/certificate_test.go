package certmanager_test

import (
	"context"
	"testing"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmanagermetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	certrepo "github.com/kyma-project/lifecycle-manager/internal/repository/certificate"
	"github.com/kyma-project/lifecycle-manager/internal/repository/certificate/certmanager"
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

func Test_Certificate_Create_Success(t *testing.T) {
	expectedCertificate := &certmanagerv1.Certificate{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       certmanagerv1.CertificateKind,
			APIVersion: certmanagerv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      certName,
			Namespace: certNamespace,
		},
		Spec: certmanagerv1.CertificateSpec{
			CommonName:  certCommonNameName,
			Duration:    &apimetav1.Duration{Duration: certDuration},
			RenewBefore: &apimetav1.Duration{Duration: certRenewBefore},
			DNSNames:    certDNSNames,
			SecretName:  certName,
			SecretTemplate: &certmanagerv1.CertificateSecretTemplate{
				Labels: k8slabels.Set{
					shared.PurposeLabel: shared.CertManager,
					shared.ManagedBy:    shared.OperatorName,
				},
			},
			IssuerRef: certmanagermetav1.ObjectReference{
				Name: issuerName,
				Kind: certmanagerv1.IssuerKind,
			},
			IsCA: false,
			Usages: []certmanagerv1.KeyUsage{
				certmanagerv1.UsageDigitalSignature,
				certmanagerv1.UsageKeyEncipherment,
			},
			PrivateKey: &certmanagerv1.CertificatePrivateKey{
				RotationPolicy: certmanagerv1.RotationPolicyAlways,
				Encoding:       certmanagerv1.PKCS1,
				Algorithm:      certmanagerv1.RSAKeyAlgorithm,
				Size:           certKeySize,
			},
		},
	}

	kcpStub := &kcpStub{}
	certClient := certmanager.NewCertificate(
		kcpStub,
		issuerName,
		certrepo.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	err := certClient.Create(t.Context(),
		certName,
		certNamespace,
		certCommonNameName,
		certDNSNames,
	)

	require.NoError(t, err)
	assert.True(t, kcpStub.patchCalled)
	assert.NotNil(t, kcpStub.patchArg)
	assert.Equal(t, expectedCertificate, kcpStub.patchArg)
}

func Test_Certificate_Create_Error(t *testing.T) {
	kcpStub := &kcpStub{
		patchErr: assert.AnError,
	}
	certClient := certmanager.NewCertificate(
		kcpStub,
		issuerName,
		certrepo.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	err := certClient.Create(t.Context(),
		certName,
		certNamespace,
		certCommonNameName,
		certDNSNames,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to patch certificate")
	assert.True(t, kcpStub.patchCalled)
}

func Test_Certificate_Delete_Success(t *testing.T) {
	kcpStub := &kcpStub{}
	certClient := certmanager.NewCertificate(
		kcpStub,
		issuerName,
		certrepo.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	err := certClient.Delete(t.Context(), certName, certNamespace)

	require.NoError(t, err)
	assert.True(t, kcpStub.deleteCalled)
	assert.NotNil(t, kcpStub.deleteArg)
	assert.Equal(t, certName, kcpStub.deleteArg.Name)
	assert.Equal(t, certNamespace, kcpStub.deleteArg.Namespace)
}

func Test_Certificate_Delete_Error(t *testing.T) {
	kcpStub := &kcpStub{
		deleteErr: assert.AnError,
	}
	certClient := certmanager.NewCertificate(
		kcpStub,
		issuerName,
		certrepo.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	err := certClient.Delete(t.Context(), certName, certNamespace)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete certificate")
	assert.True(t, kcpStub.deleteCalled)
}

func Test_Certificate_Delete_IgnoreNotFoundError(t *testing.T) {
	kcpStub := &kcpStub{
		deleteErr: apierrors.NewNotFound(certmanagerv1.Resource("certificates"), certName),
	}
	certClient := certmanager.NewCertificate(
		kcpStub,
		issuerName,
		certrepo.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	err := certClient.Delete(t.Context(), certName, certNamespace)

	require.NoError(t, err)
	assert.True(t, kcpStub.deleteCalled)
	assert.NotNil(t, kcpStub.deleteArg)
	assert.Equal(t, certName, kcpStub.deleteArg.Name)
	assert.Equal(t, certNamespace, kcpStub.deleteArg.Namespace)
}

func Test_Certificate_GetRenewalTime_Success(t *testing.T) {
	kcpStub := &kcpStub{
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
	certClient := certmanager.NewCertificate(
		kcpStub,
		issuerName,
		certrepo.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	time, err := certClient.GetRenewalTime(t.Context(), certName, certNamespace)

	require.NoError(t, err)
	assert.Equal(t, kcpStub.getCert.Status.RenewalTime.Time, time)
	assert.True(t, kcpStub.getCalled)
}

func Test_Certificate_GetRenewalTime_Error(t *testing.T) {
	kcpStub := &kcpStub{
		getErr: assert.AnError,
	}
	certClient := certmanager.NewCertificate(
		kcpStub,
		issuerName,
		certrepo.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	time, err := certClient.GetRenewalTime(t.Context(), certName, certNamespace)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get certificate")
	assert.True(t, time.IsZero())
	assert.True(t, kcpStub.getCalled)
}

func Test_Certificate_GetRenewalTime_NoRenewalTime(t *testing.T) {
	kcpStub := &kcpStub{
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
	certClient := certmanager.NewCertificate(
		kcpStub,
		issuerName,
		certrepo.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	time, err := certClient.GetRenewalTime(t.Context(), certName, certNamespace)

	require.Error(t, err)
	assert.Equal(t, certrepo.ErrNoRenewalTime, err)
	assert.True(t, time.IsZero())
	assert.True(t, kcpStub.getCalled)
}

func Test_Certificate_GetValidity_Success(t *testing.T) {
	kcpStub := &kcpStub{
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
	certClient := certmanager.NewCertificate(
		kcpStub,
		issuerName,
		certrepo.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	notBefore, notAfter, err := certClient.GetValidity(t.Context(), certName, certNamespace)

	require.NoError(t, err)
	assert.Equal(t, kcpStub.getCert.Status.NotBefore.Time, notBefore)
	assert.Equal(t, kcpStub.getCert.Status.NotAfter.Time, notAfter)
	assert.True(t, kcpStub.getCalled)
}

func Test_Certificate_GetValidity_NoNotBefore(t *testing.T) {
	kcpStub := &kcpStub{
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
	certClient := certmanager.NewCertificate(
		kcpStub,
		issuerName,
		certrepo.CertificateConfig{
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
	assert.True(t, kcpStub.getCalled)
}

func Test_Certificate_GetValidity_NoNotAfter(t *testing.T) {
	kcpStub := &kcpStub{
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
	certClient := certmanager.NewCertificate(
		kcpStub,
		issuerName,
		certrepo.CertificateConfig{
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
	assert.True(t, kcpStub.getCalled)
}

func Test_Certificate_GetValidity_GetError(t *testing.T) {
	kcpStub := &kcpStub{
		getErr: assert.AnError,
	}
	certClient := certmanager.NewCertificate(
		kcpStub,
		issuerName,
		certrepo.CertificateConfig{
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
	assert.True(t, kcpStub.getCalled)
}

// Test stubs

type kcpStub struct {
	client.Client
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

func (c *kcpStub) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	c.getCalled = true
	if c.getCert != nil {
		//nolint:forcetypeassert // test code
		c.getCert.DeepCopyInto(obj.(*certmanagerv1.Certificate))
	}
	return c.getErr
}

func (c *kcpStub) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	c.deleteCalled = true
	//nolint:forcetypeassert // test code
	c.deleteArg = obj.(*certmanagerv1.Certificate)
	return c.deleteErr
}

func (c *kcpStub) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	c.patchCalled = true
	//nolint:forcetypeassert // test code
	c.patchArg = obj.(*certmanagerv1.Certificate)
	return c.patchErr
}
