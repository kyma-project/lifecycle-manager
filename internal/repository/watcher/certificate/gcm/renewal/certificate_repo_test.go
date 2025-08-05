package renewal_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/gcm/renewal"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestGet_WhenClientReturnsError_ReturnsError(t *testing.T) {
	kcpClient := clientStub{
		getErr: assert.AnError,
	}
	certRepo := renewal.NewRepository(kcpClient, random.Name())

	result, err := certRepo.Get(t.Context(), random.Name())

	require.Error(t, err)
	require.ErrorIs(t, err, assert.AnError)
	assert.Nil(t, result)
}

type clientStub struct {
	client.Client
	getErr         error
	getCalls       int
	getLastCallArg client.ObjectKey
}

func (c clientStub) Get(_ context.Context, key client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	c.getCalls++
	c.getLastCallArg = key
	if c.getErr != nil {
		return c.getErr
	}
	return nil
}
