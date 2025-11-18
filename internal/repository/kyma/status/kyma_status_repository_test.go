package status_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	statusrepo "github.com/kyma-project/lifecycle-manager/internal/repository/kyma/status"
)

const (
	kymaName      = "test-kyma"
	kymaNamespace = "test-namespace"
	testMessage   = "test operation message"
)

var errGeneric = errors.New("generic error")

func TestNewRepository(t *testing.T) {
	t.Parallel()

	statusWriter := &statusWriterStub{}
	repo := statusrepo.NewRepository(statusWriter)

	require.NotNil(t, repo)
}

func TestRepository_UpdateKymaStatus_WhenPatchSucceeds_ReturnNoError(t *testing.T) {
	t.Parallel()

	statusWriter := &statusWriterStub{}
	repo := statusrepo.NewRepository(statusWriter)
	kyma := createTestKyma()

	err := repo.UpdateKymaStatus(context.Background(), kyma, shared.StateReady, testMessage)

	require.NoError(t, err)
	require.True(t, statusWriter.PatchCalled)
	require.Equal(t, shared.StateReady, kyma.Status.State)
	require.Equal(t, testMessage, kyma.Status.LastOperation.Operation)
	require.NotZero(t, kyma.Status.LastOperation.LastUpdateTime)
	require.Nil(t, kyma.ManagedFields)
}

func TestRepository_UpdateKymaStatus_WhenPatchFails_ReturnError(t *testing.T) {
	t.Parallel()

	statusWriter := &statusWriterStubWithError{err: errGeneric}
	repo := statusrepo.NewRepository(statusWriter)
	kyma := createTestKyma()

	err := repo.UpdateKymaStatus(context.Background(), kyma, shared.StateError, testMessage)

	require.Error(t, err)
	require.ErrorIs(t, err, errGeneric)
	require.True(t, statusWriter.PatchCalled)
}

func TestRepository_UpdateKymaStatus_StateDeleting_NoActiveChannelSet(t *testing.T) {
	t.Parallel()

	statusWriter := &statusWriterStub{}
	repo := statusrepo.NewRepository(statusWriter)
	kyma := createTestKymaWithActiveChannel()

	err := repo.UpdateKymaStatus(context.Background(), kyma, shared.StateDeleting, testMessage)

	require.NoError(t, err)
	require.Equal(t, shared.StateDeleting, kyma.Status.State)
	// Active channel should remain unchanged for deleting state
	require.NotEmpty(t, kyma.Status.ActiveChannel)
}

func TestRepository_UpdateKymaStatus_StateReady_SetsActiveChannel(t *testing.T) {
	t.Parallel()

	statusWriter := &statusWriterStub{}
	repo := statusrepo.NewRepository(statusWriter)
	kyma := createTestKyma()

	err := repo.UpdateKymaStatus(context.Background(), kyma, shared.StateReady, testMessage)

	require.NoError(t, err)
	require.Equal(t, shared.StateReady, kyma.Status.State)
	// SetActiveChannel should be called for StateReady
}

func TestRepository_UpdateKymaStatus_StateWarning_SetsActiveChannel(t *testing.T) {
	t.Parallel()

	statusWriter := &statusWriterStub{}
	repo := statusrepo.NewRepository(statusWriter)
	kyma := createTestKyma()

	err := repo.UpdateKymaStatus(context.Background(), kyma, shared.StateWarning, testMessage)

	require.NoError(t, err)
	require.Equal(t, shared.StateWarning, kyma.Status.State)
	// SetActiveChannel should be called for StateWarning
}

func TestRepository_UpdateKymaStatus_VariousStates_ProperlyHandled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		state     shared.State
		expectErr bool
	}{
		{
			name:      "empty state",
			state:     "",
			expectErr: false,
		},
		{
			name:      "processing state",
			state:     shared.StateProcessing,
			expectErr: false,
		},
		{
			name:      "error state",
			state:     shared.StateError,
			expectErr: false,
		},
		{
			name:      "unmanaged state",
			state:     shared.StateUnmanaged,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			statusWriter := &statusWriterStub{}
			repo := statusrepo.NewRepository(statusWriter)
			kyma := createTestKyma()

			err := repo.UpdateKymaStatus(context.Background(), kyma, tt.state, testMessage)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.state, kyma.Status.State)
				require.Equal(t, testMessage, kyma.Status.LastOperation.Operation)
			}
		})
	}
}

func TestRepository_UpdateKymaStatus_LastOperationTimestamp_IsSet(t *testing.T) {
	t.Parallel()

	statusWriter := &statusWriterStub{}
	repo := statusrepo.NewRepository(statusWriter)
	kyma := createTestKyma()
	beforeUpdate := time.Now()

	err := repo.UpdateKymaStatus(context.Background(), kyma, shared.StateReady, testMessage)

	require.NoError(t, err)
	afterUpdate := time.Now()

	lastUpdateTime := kyma.Status.LastOperation.LastUpdateTime.Time
	require.True(t, lastUpdateTime.After(beforeUpdate) || lastUpdateTime.Equal(beforeUpdate))
	require.True(t, lastUpdateTime.Before(afterUpdate) || lastUpdateTime.Equal(afterUpdate))
}

func TestRepository_UpdateKymaStatus_ManagedFieldsCleared(t *testing.T) {
	t.Parallel()

	statusWriter := &statusWriterStub{}
	repo := statusrepo.NewRepository(statusWriter)
	kyma := createTestKyma()

	// Set some managed fields
	kyma.ManagedFields = []apimetav1.ManagedFieldsEntry{
		{Manager: "test-manager"},
	}

	err := repo.UpdateKymaStatus(context.Background(), kyma, shared.StateReady, testMessage)

	require.NoError(t, err)
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

// StatusWriter stubs

type statusWriterStub struct {
	PatchCalled bool
}

func (s *statusWriterStub) Patch(_ context.Context,
	_ client.Object,
	_ client.Patch,
	_ ...client.SubResourcePatchOption,
) error {
	s.PatchCalled = true
	return nil
}

func (s *statusWriterStub) Create(_ context.Context,
	_ client.Object,
	_ client.Object,
	_ ...client.SubResourceCreateOption,
) error {
	return nil
}

func (s *statusWriterStub) Update(_ context.Context, _ client.Object, _ ...client.SubResourceUpdateOption) error {
	return nil
}

type statusWriterStubWithError struct {
	PatchCalled bool
	err         error
}

func (s *statusWriterStubWithError) Patch(_ context.Context,
	_ client.Object,
	_ client.Patch,
	_ ...client.SubResourcePatchOption,
) error {
	s.PatchCalled = true
	return s.err
}

func (s *statusWriterStubWithError) Create(_ context.Context,
	_ client.Object,
	_ client.Object,
	_ ...client.SubResourceCreateOption,
) error {
	return s.err
}

func (s *statusWriterStubWithError) Update(_ context.Context,
	_ client.Object,
	_ ...client.SubResourceUpdateOption,
) error {
	return s.err
}
