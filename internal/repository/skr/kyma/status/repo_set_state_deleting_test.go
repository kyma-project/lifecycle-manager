package status_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
	"github.com/kyma-project/lifecycle-manager/internal/errors"
	"github.com/kyma-project/lifecycle-manager/internal/repository/skr/kyma/status"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestSetStateDeleting_ClientCallSucceeds(t *testing.T) {
	kcpKymaName := types.NamespacedName{Name: random.Name(), Namespace: random.Name()}

	statusClientStub := &statusClientStub{}
	clientStub := &clientStub{
		status: statusClientStub,
	}
	clientCacheStub := &skrClientCacheStub{
		client: clientStub,
	}

	repo := status.NewRepository(clientCacheStub)

	err := repo.SetStateDeleting(t.Context(), kcpKymaName)

	require.NoError(t, err)
	assert.True(t, statusClientStub.called)
	assert.NotNil(t, statusClientStub.patchedObject)
	// kcpKymaName used to get the client
	assert.Equal(t, kcpKymaName, clientCacheStub.receivedKey)
	// standard Kyma name used to get the Kyma from SKR
	assert.Equal(t, shared.DefaultRemoteKymaName, statusClientStub.patchedObject.Name)
	assert.Equal(t, shared.DefaultRemoteNamespace, statusClientStub.patchedObject.Namespace)
	// correct state set
	assert.Equal(t, shared.StateDeleting, statusClientStub.patchedObject.Status.State)
	assert.Equal(t, ".status.State set to Deleting", statusClientStub.patchedObject.Status.Operation)
	assert.WithinDuration(t,
		time.Now(),
		statusClientStub.patchedObject.Status.LastUpdateTime.Time,
		time.Second)
	// correct options used
	assert.Contains(t, statusClientStub.opts, fieldowners.LifecycleManager)
	assert.Contains(t, statusClientStub.opts, client.ForceOwnership)
}

func TestSetStateDeleting_ClientReturnsAnError(t *testing.T) {
	statusClientStub := &statusClientStub{
		err: assert.AnError,
	}
	clientStub := &clientStub{
		status: statusClientStub,
	}
	clientCacheStub := &skrClientCacheStub{
		client: clientStub,
	}

	repo := status.NewRepository(clientCacheStub)

	err := repo.SetStateDeleting(context.Background(),
		types.NamespacedName{
			Name:      random.Name(),
			Namespace: random.Name(),
		})

	require.ErrorIs(t, err, assert.AnError)
	assert.True(t, statusClientStub.called)
}

func TestSetStateDeleting_ClientNotFound(t *testing.T) {
	repo := status.NewRepository(&skrClientCacheStub{
		client: nil, // No client available in the cache
	})

	err := repo.SetStateDeleting(context.Background(),
		types.NamespacedName{
			Name:      random.Name(),
			Namespace: random.Name(),
		})

	require.Error(t, err)
	require.ErrorIs(t, err, errors.ErrSkrClientNotFound)
}

type clientStub struct {
	client.Client

	status client.SubResourceWriter
}

func (c *clientStub) Status() client.SubResourceWriter {
	return c.status
}

type statusClientStub struct {
	client.SubResourceWriter

	err error

	called        bool
	patchedObject *v1beta2.Kyma
	opts          []client.SubResourcePatchOption
}

func (c *statusClientStub) Patch(_ context.Context,
	obj client.Object,
	_ client.Patch,
	opts ...client.SubResourcePatchOption,
) error {
	c.called = true
	c.patchedObject = obj.(*v1beta2.Kyma)
	c.opts = opts

	return c.err
}
