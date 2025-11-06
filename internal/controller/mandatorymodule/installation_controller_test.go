package mandatorymodule_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/controller/mandatorymodule"
	"github.com/kyma-project/lifecycle-manager/internal/errors/mandatorymodule/installation"
	"github.com/stretchr/testify/require"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestInstallationReconciler_Reconcile_WhenKymaSkipReconciliation_DoesntRequeue(t *testing.T) {
	t.Parallel()

	mockInstallationService := &mockMrmInstallationService{
		HandleInstallationError: installation.ErrSkippingReconciliationKyma,
	}
	reconciler := mandatorymodule.NewInstallationReconciler(mockInstallationService, getRequeueIntervals())

	kyma := &v1beta2.Kyma{}

	result, err := reconciler.Reconcile(context.Background(), kyma)
	require.True(t, mockInstallationService.HandleInstallationCalled)
	require.NoError(t, err)
	require.Equal(t, ctrl.Result{}, result)
}

func TestInstallationReconciler_Reconcile_WhenHandleInstallationSucceeds_RequeuesAfterSuccessInterval(t *testing.T) {
	t.Parallel()

	mockInstallationService := &mockMrmInstallationService{}
	reconciler := mandatorymodule.NewInstallationReconciler(mockInstallationService, getRequeueIntervals())

	kyma := &v1beta2.Kyma{}

	result, err := reconciler.Reconcile(context.Background(), kyma)
	require.True(t, mockInstallationService.HandleInstallationCalled)
	require.NoError(t, err)
	require.Equal(t, successRequeueInterval, result.RequeueAfter)
}

func TestInstallationReconciler_Reconcile_WhenHandleInstallationFails_RequeuesWithBackoff(t *testing.T) {
	t.Parallel()

	installationErr := errors.New("installation error")
	mockInstallationService := &mockMrmInstallationService{
		HandleInstallationError: installationErr,
	}
	reconciler := mandatorymodule.NewInstallationReconciler(mockInstallationService, getRequeueIntervals())

	kyma := &v1beta2.Kyma{}

	result, err := reconciler.Reconcile(context.Background(), kyma)
	require.True(t, mockInstallationService.HandleInstallationCalled)
	require.ErrorIs(t, err, installationErr)
	require.Equal(t, ctrl.Result{}, result)
}

type mockMrmInstallationService struct {
	HandleInstallationCalled bool
	HandleInstallationError  error
}

func (m *mockMrmInstallationService) HandleInstallation(_ context.Context, _ *v1beta2.Kyma) error {
	m.HandleInstallationCalled = true
	return m.HandleInstallationError
}
