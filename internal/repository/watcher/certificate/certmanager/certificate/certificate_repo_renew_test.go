package certificate_test

import (
	"context"
	"testing"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmanagermetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	certmanagercertificate "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/certmanager/certificate" //nolint:revive // not for import
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/config"
)

func TestRenew_Success(t *testing.T) {
	clientStub := &renewClientStub{
		certificate: &certmanagerv1.Certificate{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:       certName,
				Namespace:  certNamespace,
				Generation: 4711,
			},
		},
	}
	certificateRepository, err := certmanagercertificate.NewRepository(
		clientStub,
		issuerName,
		config.CertificateValues{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
			Namespace:   certNamespace,
		},
	)
	require.NoError(t, err)

	err = certificateRepository.Renew(context.Background(), certName)

	require.NoError(t, err)
	assert.True(t, clientStub.getCalled)
	assert.True(t, clientStub.statusUpdateCalled)
	assert.Equal(t, certName, clientStub.getCalledKey.Name)
	assert.Equal(t, certNamespace, clientStub.getCalledKey.Namespace)
	assert.Len(t, clientStub.statusReceivedObj.Status.Conditions, 1)
	assert.Equal(t,
		certmanagerv1.CertificateConditionIssuing,
		clientStub.statusReceivedObj.Status.Conditions[0].Type,
	)
	assert.Equal(t, "ManuallyTriggered", clientStub.statusReceivedObj.Status.Conditions[0].Reason)
	assert.Equal(t,
		"Certificate re-issuance manually triggered",
		clientStub.statusReceivedObj.Status.Conditions[0].Message,
	)
	assert.Equal(t,
		certmanagermetav1.ConditionTrue,
		clientStub.statusReceivedObj.Status.Conditions[0].Status,
	)
	assert.Equal(t, int64(4711), clientStub.statusReceivedObj.Generation)
}

func TestRenew_GetCertificateError(t *testing.T) {
	clientStub := &renewClientStub{
		getErr: assert.AnError,
	}
	certificateRepository, err := certmanagercertificate.NewRepository(
		clientStub,
		issuerName,
		config.CertificateValues{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
			Namespace:   certNamespace,
		},
	)
	require.NoError(t, err)

	err = certificateRepository.Renew(context.Background(), certName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get certificate")
	assert.True(t, clientStub.getCalled)
	assert.False(t, clientStub.statusUpdateCalled)
}

func TestRenew_StatusUpdateError(t *testing.T) {
	clientStub := &renewClientStub{
		certificate: &certmanagerv1.Certificate{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      certName,
				Namespace: certNamespace,
			},
		},
		statusUpdateErr: assert.AnError,
	}
	certificateRepository, err := certmanagercertificate.NewRepository(
		clientStub,
		issuerName,
		config.CertificateValues{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
			Namespace:   certNamespace,
		},
	)
	require.NoError(t, err)

	err = certificateRepository.Renew(context.Background(), certName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update certificate")
	assert.True(t, clientStub.getCalled)
	assert.True(t, clientStub.statusUpdateCalled)
}

type renewClientStub struct {
	client.Client

	certificate        *certmanagerv1.Certificate
	getCalled          bool
	getCalledKey       client.ObjectKey
	getErr             error
	statusUpdateCalled bool
	statusUpdateErr    error
	statusReceivedObj  certmanagerv1.Certificate
}

func (r *renewClientStub) Get(_ context.Context,
	key client.ObjectKey,
	obj client.Object,
	_ ...client.GetOption,
) error {
	r.getCalled = true
	r.getCalledKey = key

	if r.getErr != nil {
		return r.getErr
	}

	if r.certificate != nil {
		cert, ok := obj.(*certmanagerv1.Certificate)
		if ok {
			r.certificate.DeepCopyInto(cert)
		}
	}

	return nil
}

func (r *renewClientStub) Status() client.SubResourceWriter {
	return &statusWriterStub{
		updateErr:   &r.statusUpdateErr,
		called:      &r.statusUpdateCalled,
		receivedObj: &r.statusReceivedObj,
	}
}

type statusWriterStub struct {
	client.SubResourceWriter

	updateErr   *error
	called      *bool
	receivedObj *certmanagerv1.Certificate
}

func (s *statusWriterStub) Update(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
	*s.called = true
	if *s.updateErr != nil {
		return *s.updateErr
	}

	*s.receivedObj = *(obj.(*certmanagerv1.Certificate))

	return nil
}
