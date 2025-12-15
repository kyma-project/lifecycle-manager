package certificate_test

import (
	"context"
	"testing"

	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/config"
	gcmcertificate "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/gcm/certificate"
)

func TestExists_WhenCertificateExists_ReturnsTrue(t *testing.T) {
	clientStub := &existsClientStub{
		certificate: &gcertv1alpha1.Certificate{},
	}
	certificateRepository, err := gcmcertificate.NewRepository(
		clientStub,
		issuerName,
		issuerNamespace,
		config.CertificateValues{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     int(certKeySize),
			Namespace:   certNamespace,
		},
	)
	require.NoError(t, err)

	exists, err := certificateRepository.Exists(context.Background(), certName)

	require.NoError(t, err)
	assert.True(t, exists)
	assert.True(t, clientStub.getCalled)
	assert.Equal(t, certName, clientStub.getCalledKey.Name)
	assert.Equal(t, certNamespace, clientStub.getCalledKey.Namespace)
}

func TestExists_WhenCertificateDoesNotExist_ReturnsFalse(t *testing.T) {
	clientStub := &existsClientStub{
		getErr: apierrors.NewNotFound(schema.GroupResource{
			Group:    gcertv1alpha1.SchemeGroupVersion.Group,
			Resource: "certificates",
		}, certName),
	}
	certificateRepository, err := gcmcertificate.NewRepository(
		clientStub,
		issuerName,
		issuerNamespace,
		config.CertificateValues{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     int(certKeySize),
			Namespace:   certNamespace,
		},
	)
	require.NoError(t, err)

	exists, err := certificateRepository.Exists(context.Background(), certName)

	require.NoError(t, err)
	assert.False(t, exists)
	assert.True(t, clientStub.getCalled)
}

func TestExists_WhenClientReturnsOtherError_ReturnsError(t *testing.T) {
	clientStub := &existsClientStub{
		getErr: assert.AnError,
	}
	certificateRepository, err := gcmcertificate.NewRepository(
		clientStub,
		issuerName,
		issuerNamespace,
		config.CertificateValues{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     int(certKeySize),
			Namespace:   certNamespace,
		},
	)
	require.NoError(t, err)

	exists, err := certificateRepository.Exists(context.Background(), certName)

	require.Error(t, err)
	assert.False(t, exists)
	assert.True(t, clientStub.getCalled)
	assert.Contains(t, err.Error(), "failed to check existence of certificate")
}

type existsClientStub struct {
	client.Client

	certificate  *gcertv1alpha1.Certificate
	getCalled    bool
	getCalledKey client.ObjectKey
	getErr       error
}

func (e *existsClientStub) Get(_ context.Context,
	key client.ObjectKey,
	obj client.Object,
	_ ...client.GetOption,
) error {
	e.getCalled = true
	e.getCalledKey = key

	if e.getErr != nil {
		return e.getErr
	}

	if e.certificate != nil {
		cert, ok := obj.(*gcertv1alpha1.Certificate)
		if ok {
			e.certificate.DeepCopyInto(cert)
		}
	}

	return nil
}
