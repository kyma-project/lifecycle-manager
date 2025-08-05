package gcm_test

import (
	"context"
	"testing"

	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate/renewal/gcm"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestRenew_WhenRepoReturnsError_ReturnsError(t *testing.T) {
	certRepo := &certRepoStub{
		getErr: assert.AnError,
	}
	service := gcm.NewService(certRepo)
	certName := random.Name()

	err := service.Renew(t.Context(), certName)

	require.ErrorIs(t, err, assert.AnError)
	require.Contains(t, err.Error(), "could not get certificate for renewal")
	require.Equal(t, 1, certRepo.getCalls)
	require.Equal(t, certName, certRepo.getLastCallArg)
	require.Equal(t, 0, certRepo.updateCalls)
}

func TestRenew_WhenRepoReturnsNil_ReturnsError(t *testing.T) {
	certRepo := &certRepoStub{}
	service := gcm.NewService(certRepo)
	certName := random.Name()

	err := service.Renew(t.Context(), certName)

	require.Error(t, err)
	require.Contains(t, err.Error(), "could not get certificate for renewal")
	require.Equal(t, 1, certRepo.getCalls)
	require.Equal(t, certName, certRepo.getLastCallArg)
	require.Equal(t, 0, certRepo.updateCalls)
}

func TestRenew_WhenRepoReturnsCert_CallsRepoUpdateWithSpecRenewTrue(t *testing.T) {
	certRepo := &certRepoStub{
		getReturnValue: &gcertv1alpha1.Certificate{
			Spec: gcertv1alpha1.CertificateSpec{
				EnsureRenewedAfter: nil,
				Renew:              nil,
			},
		},
	}
	service := gcm.NewService(certRepo)
	certName := random.Name()

	err := service.Renew(t.Context(), certName)

	require.NoError(t, err)
	require.Equal(t, 1, certRepo.getCalls)
	require.Equal(t, certName, certRepo.getLastCallArg)
	require.Equal(t, 1, certRepo.updateCalls)
	require.True(t, *certRepo.getReturnValue.Spec.Renew)
	require.Nil(t, certRepo.getReturnValue.Spec.EnsureRenewedAfter)
}

func TestRenew_WhenRepoReturnsCert_CallsRepoUpdateWithSpecEnsureRenewedAfterNil(t *testing.T) {
	certRepo := &certRepoStub{
		getReturnValue: &gcertv1alpha1.Certificate{
			Spec: gcertv1alpha1.CertificateSpec{
				EnsureRenewedAfter: &apimetav1.Time{},
				Renew:              nil,
			},
		},
	}
	service := gcm.NewService(certRepo)
	certName := random.Name()

	err := service.Renew(t.Context(), certName)

	require.NoError(t, err)
	require.Equal(t, 1, certRepo.getCalls)
	require.Equal(t, certName, certRepo.getLastCallArg)
	require.Equal(t, 1, certRepo.updateCalls)
	require.True(t, *certRepo.getReturnValue.Spec.Renew)
	require.Nil(t, certRepo.getReturnValue.Spec.EnsureRenewedAfter)
}

func TestRenew_WhenRepoUpdateReturnsError_ReturnsError(t *testing.T) {
	certRepo := &certRepoStub{
		getReturnValue: &gcertv1alpha1.Certificate{
			Spec: gcertv1alpha1.CertificateSpec{},
		},
		updateErr: assert.AnError,
	}
	service := gcm.NewService(certRepo)
	certName := random.Name()

	err := service.Renew(t.Context(), certName)

	require.ErrorIs(t, err, assert.AnError)
	require.Contains(t, err.Error(), "failed to update certificate for renewal")
}

type certRepoStub struct {
	getCalls          int
	getLastCallArg    string
	getErr            error
	getReturnValue    *gcertv1alpha1.Certificate
	updateCalls       int
	updateLastCallArg *gcertv1alpha1.Certificate
	updateErr         error
}

func (c *certRepoStub) Get(_ context.Context, name string) (*gcertv1alpha1.Certificate, error) {
	c.getCalls++
	c.getLastCallArg = name
	if c.getErr != nil {
		return nil, c.getErr
	}
	return c.getReturnValue, nil
}

func (c *certRepoStub) Update(_ context.Context, cert *gcertv1alpha1.Certificate) error {
	c.updateCalls++
	c.updateLastCallArg = cert
	if c.updateErr != nil {
		return c.updateErr
	}
	return nil
}
