package status_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiconfigsv1beta2 "github.com/kyma-project/lifecycle-manager/api/applyconfigurations/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
	errorsinternal "github.com/kyma-project/lifecycle-manager/internal/errors"
	skrkymastatusrepo "github.com/kyma-project/lifecycle-manager/internal/repository/skr/kyma/status"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestSetStateDeleting_ClientCallSucceeds(t *testing.T) {
	kcpKymaName := types.NamespacedName{Name: random.Name(), Namespace: random.Name()}

	statusClientStub := &statusClientStub{}
	clientStub := &clientStub{
		status: statusClientStub,
	}
	clientRetrieverStub := &skrClientRetrieverStub{
		client: clientStub,
	}

	repo := skrkymastatusrepo.NewRepository(clientRetrieverStub.retrieverFunc())

	err := repo.SetStateDeleting(t.Context(), kcpKymaName)

	require.NoError(t, err)
	assert.True(t, statusClientStub.called)
	require.NotNil(t, statusClientStub.appliedConfig)
	require.NotNil(t, statusClientStub.appliedConfig.Status)
	// kcpKymaName used to get the client
	assert.Equal(t, kcpKymaName, clientRetrieverStub.receivedKey)
	// standard Kyma name used to get the Kyma from SKR
	assert.Equal(t, shared.DefaultRemoteKymaName, *statusClientStub.appliedConfig.GetName())
	assert.Equal(t, shared.DefaultRemoteNamespace, *statusClientStub.appliedConfig.GetNamespace())
	// correct state set
	assert.Equal(t, shared.StateDeleting, *statusClientStub.appliedConfig.Status.State)
	assert.Equal(t, ".status.State set to Deleting", statusClientStub.appliedConfig.Status.LastOperation.Operation)
	assert.WithinDuration(t,
		time.Now(),
		statusClientStub.appliedConfig.Status.LastOperation.LastUpdateTime.Time,
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
	clientRetrieverStub := &skrClientRetrieverStub{
		client: clientStub,
	}

	repo := skrkymastatusrepo.NewRepository(clientRetrieverStub.retrieverFunc())

	err := repo.SetStateDeleting(context.Background(),
		types.NamespacedName{
			Name:      random.Name(),
			Namespace: random.Name(),
		})

	require.ErrorIs(t, err, assert.AnError)
	assert.True(t, statusClientStub.called)
}

func TestSetStateDeleting_ClientNotFound(t *testing.T) {
	clientRetrieverStub := &skrClientRetrieverStub{
		client: nil, // No client available in the cache
	}
	repo := skrkymastatusrepo.NewRepository(clientRetrieverStub.retrieverFunc())

	err := repo.SetStateDeleting(context.Background(),
		types.NamespacedName{
			Name:      random.Name(),
			Namespace: random.Name(),
		})

	require.Error(t, err)
	require.ErrorIs(t, err, errorsinternal.ErrSkrClientNotFound)
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
	appliedConfig *apiconfigsv1beta2.KymaApplyConfiguration
	opts          []client.SubResourceApplyOption
}

func (c *statusClientStub) Apply(_ context.Context,
	obj machineryruntime.ApplyConfiguration,
	opts ...client.SubResourceApplyOption,
) error {
	c.called = true
	c.appliedConfig = obj.(*apiconfigsv1beta2.KymaApplyConfiguration)
	c.opts = opts

	return c.err
}
