package status_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/repository/kyma/status"
)

const (
	kymaName                = "test-kyma"
	kymaNamespace           = "test-namespace"
	expectedDeletingMessage = "waiting for modules to be deleted"
)

func TestNewRepository(t *testing.T) {
	t.Parallel()

	statusWriter := &statusWriterStub{}
	repo := status.NewRepository(statusWriter)

	require.NotNil(t, repo)
}

func TestRepository_UpdateStatusDeleting_WhenPatchSucceeds_ReturnNoError(t *testing.T) {
	t.Parallel()

	statusWriter := &statusWriterStub{}
	repo := status.NewRepository(statusWriter)
	kyma := createTestKyma()

	err := repo.UpdateStatusDeleting(context.Background(), kyma)

	require.NoError(t, err)
	require.True(t, statusWriter.PatchCalled)
	require.Equal(t, shared.StateDeleting, kyma.Status.State)
	require.Equal(t, expectedDeletingMessage, kyma.Status.Operation)
	require.NotZero(t, kyma.Status.LastUpdateTime)
	require.Nil(t, kyma.ManagedFields)
}

func TestRepository_UpdateStatusDeleting_WhenPatchFails_ReturnError(t *testing.T) {
	t.Parallel()

	statusWriter := &statusWriterStub{err: assert.AnError}
	repo := status.NewRepository(statusWriter)
	kyma := createTestKyma()

	err := repo.UpdateStatusDeleting(context.Background(), kyma)

	require.Error(t, err)
	require.ErrorIs(t, err, assert.AnError)
	require.True(t, statusWriter.PatchCalled)
}

func TestRepository_UpdateStatusDeleting_NoActiveChannelModification(t *testing.T) {
	t.Parallel()

	statusWriter := &statusWriterStub{}
	repo := status.NewRepository(statusWriter)
	kyma := createTestKymaWithActiveChannel()
	originalChannel := kyma.Status.ActiveChannel

	err := repo.UpdateStatusDeleting(context.Background(), kyma)

	require.NoError(t, err)
	require.Equal(t, shared.StateDeleting, kyma.Status.State)
	require.Equal(t, expectedDeletingMessage, kyma.Status.Operation)
	// Active channel should remain unchanged for deleting state
	require.Equal(t, originalChannel, kyma.Status.ActiveChannel)
}

func TestRepository_UpdateStatusDeleting_TimestampIsSet(t *testing.T) {
	t.Parallel()

	statusWriter := &statusWriterStub{}
	repo := status.NewRepository(statusWriter)
	kyma := createTestKyma()
	beforeUpdate := time.Now()

	err := repo.UpdateStatusDeleting(context.Background(), kyma)

	require.NoError(t, err)
	afterUpdate := time.Now()

	lastUpdateTime := kyma.Status.LastUpdateTime.Time
	require.True(t, lastUpdateTime.After(beforeUpdate) || lastUpdateTime.Equal(beforeUpdate))
	require.True(t, lastUpdateTime.Before(afterUpdate) || lastUpdateTime.Equal(afterUpdate))
}

func TestRepository_UpdateStatusDeleting_ManagedFieldsCleared(t *testing.T) {
	t.Parallel()

	statusWriter := &statusWriterStub{}
	repo := status.NewRepository(statusWriter)
	kyma := createTestKyma()

	// Set some managed fields
	kyma.ManagedFields = []apimetav1.ManagedFieldsEntry{
		{Manager: "test-manager"},
	}

	err := repo.UpdateStatusDeleting(context.Background(), kyma)

	require.NoError(t, err)
	require.Equal(t, shared.StateDeleting, kyma.Status.State)
	require.Equal(t, expectedDeletingMessage, kyma.Status.Operation)
	require.Nil(t, kyma.ManagedFields)
}

// Helper functions

func createTestKyma() *v1beta2.Kyma {
	return &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      kymaName,
			Namespace: kymaNamespace,
		},
		Status: v1beta2.KymaStatus{
			State: shared.StateProcessing,
		},
	}
}

func createTestKymaWithActiveChannel() *v1beta2.Kyma {
	kyma := createTestKyma()
	kyma.Status.ActiveChannel = "stable"
	return kyma
}

type statusWriterStub struct {
	client.StatusWriter

	err         error
	PatchCalled bool
}

func (s *statusWriterStub) Patch(_ context.Context,
	_ client.Object,
	_ client.Patch,
	_ ...client.SubResourcePatchOption,
) error {
	s.PatchCalled = true
	if s.err != nil {
		return s.err
	}
	return nil
}
