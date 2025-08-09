package renewal_test

import (
	"context"
	"testing"

	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/gcm/renewal"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestGet_WhenClientReturnsError_ReturnsError(t *testing.T) {
	kcpClient := &clientStub{
		getErr: assert.AnError,
	}
	certRepo := renewal.NewRepository(kcpClient, random.Name())

	result, err := certRepo.Get(t.Context(), random.Name())

	require.ErrorIs(t, err, assert.AnError)
	require.ErrorContains(t, err, "failed to get GCM Certificate")
	assert.Nil(t, result)
}

func TestGet_WhenClientReturnsCertificate_ReturnsCertificate(t *testing.T) {
	kcpClient := &clientStub{}
	certNamespace := random.Name()
	certRepo := renewal.NewRepository(kcpClient, certNamespace)
	certName := random.Name()

	result, err := certRepo.Get(t.Context(), certName)

	require.NoError(t, err)
	assert.Equal(t, certName, result.Name)
	assert.Equal(t, certNamespace, result.Namespace)
	assert.Equal(t, 1, kcpClient.getCalls)
	assert.Equal(t, client.ObjectKey{Name: certName, Namespace: certNamespace}, kcpClient.getLastCallArg)
}

func TestUpdate_WhenClientReturnsError_ReturnsError(t *testing.T) {
	kcpClient := &clientStub{
		updateErr: assert.AnError,
	}
	certRepo := renewal.NewRepository(kcpClient, random.Name())

	err := certRepo.Update(t.Context(), &gcertv1alpha1.Certificate{})

	require.ErrorIs(t, err, assert.AnError)
	require.ErrorContains(t, err, "failed to update GCM Certificate")
}

func TestUpdate_WhenCalledWithNilCert_ReturnsError(t *testing.T) {
	kcpClient := &clientStub{}
	certRepo := renewal.NewRepository(kcpClient, random.Name())

	err := certRepo.Update(t.Context(), nil)

	require.ErrorIs(t, err, renewal.ErrNilCertificate)
	assert.Equal(t, 0, kcpClient.updateCalls)
}

func TestUpdate_WhenClientCallSucceeds_Returns(t *testing.T) {
	kcpClient := &clientStub{}
	certRepo := renewal.NewRepository(kcpClient, random.Name())

	err := certRepo.Update(t.Context(), &gcertv1alpha1.Certificate{})

	require.NoError(t, err)
	assert.Equal(t, 1, kcpClient.updateCalls)
}

type clientStub struct {
	client.Client
	getErr         error
	getCalls       int
	getLastCallArg client.ObjectKey
	updateErr      error
	updateCalls    int
	updateCallArg  client.Object
}

func (c *clientStub) Get(_ context.Context, key client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	c.getCalls++
	c.getLastCallArg = key
	if c.getErr != nil {
		return c.getErr
	}
	return nil
}

func (c *clientStub) Update(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
	c.updateCalls++
	c.updateCallArg = obj
	if c.updateErr != nil {
		return c.updateErr
	}
	return nil
}
