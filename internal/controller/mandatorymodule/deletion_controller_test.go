package mandatorymodule_test

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/controller/mandatorymodule"
	"github.com/kyma-project/lifecycle-manager/internal/errors/mandatorymodule/deletion"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
)

// nolint:gosec // This is only test code
var successRequeueInterval = time.Duration(rand.Intn(10)) * time.Second

func TestDeletionReconciler_Reconcile_WhenMrmNotInDeletingState_DoesntRequeue(t *testing.T) {
	t.Parallel()

	mockDeletionService := &mockMrmDeletionService{
		HandleDeletionError: deletion.ErrMrmNotInDeletingState,
	}
	reconciler := mandatorymodule.NewDeletionReconciler(mockDeletionService, getRequeueIntervals())

	mrm := &v1beta2.ModuleReleaseMeta{}

	result, err := reconciler.Reconcile(context.Background(), mrm)
	require.True(t, mockDeletionService.HandleDeletionCalled)
	require.NoError(t, err)
	require.Equal(t, ctrl.Result{}, result)
}

func TestDeletionReconciler_Reconcile_WhenHandleDeletionSucceeds_RequeuesAfterSuccessInterval(t *testing.T) {
	t.Parallel()

	mockDeletionService := &mockMrmDeletionService{}
	reconciler := mandatorymodule.NewDeletionReconciler(mockDeletionService, getRequeueIntervals())

	mrm := &v1beta2.ModuleReleaseMeta{}

	result, err := reconciler.Reconcile(context.Background(), mrm)
	require.True(t, mockDeletionService.HandleDeletionCalled)
	require.NoError(t, err)
	require.Equal(t, successRequeueInterval, result.RequeueAfter)
}

func TestDeletionReconciler_Reconcile_WhenHandleDeletionFails_RequeuesWithBackoff(t *testing.T) {
	t.Parallel()

	deletionErr := errors.New("deletion error")
	mockDeletionService := &mockMrmDeletionService{
		HandleDeletionError: deletionErr,
	}
	reconciler := mandatorymodule.NewDeletionReconciler(mockDeletionService, getRequeueIntervals())

	mrm := &v1beta2.ModuleReleaseMeta{}

	result, err := reconciler.Reconcile(context.Background(), mrm)
	require.True(t, mockDeletionService.HandleDeletionCalled)
	require.ErrorIs(t, err, deletionErr)
	require.Equal(t, ctrl.Result{}, result)
}

func getRequeueIntervals() queue.RequeueIntervals {
	return queue.RequeueIntervals{
		Success: successRequeueInterval,
	}
}

type mockMrmDeletionService struct {
	HandleDeletionCalled bool
	HandleDeletionError  error
}

func (m *mockMrmDeletionService) HandleDeletion(_ context.Context, _ *v1beta2.ModuleReleaseMeta) error {
	m.HandleDeletionCalled = true
	return m.HandleDeletionError
}
