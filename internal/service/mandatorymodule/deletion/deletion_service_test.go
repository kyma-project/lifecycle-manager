package deletion_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule/deletion"
)

func TestDeletionService_HandleDeletion_ExecutionOrder(t *testing.T) {
	t.Parallel()

	var executionOrder []string

	ensureFinalizerStub := &UseCaseStub{UseCaseName: "ensureFinalizer", ExecutionOrder: &executionOrder}
	skipNonDeletingStub := &UseCaseStub{UseCaseName: "skipNonDeleting", ExecutionOrder: &executionOrder}
	deleteManifestsStub := &UseCaseStub{UseCaseName: "deleteManifests", ExecutionOrder: &executionOrder}
	removeFinalizerStub := &UseCaseStub{UseCaseName: "removeFinalizer", ExecutionOrder: &executionOrder}

	service := deletion.NewService(
		ensureFinalizerStub,
		skipNonDeletingStub,
		deleteManifestsStub,
		removeFinalizerStub,
	)
	mrm := &v1beta2.ModuleReleaseMeta{}

	for range 5 {
		err := service.HandleDeletion(context.Background(), mrm)
		require.NoError(t, err)
	}

	expectedOrder := []string{
		"ensureFinalizer",
		"skipNonDeleting",
		"deleteManifests",
		"removeFinalizer",
	}
	require.Equal(t, expectedOrder, executionOrder)

	require.True(t, ensureFinalizerStub.IsApplicableCalled)
	require.True(t, ensureFinalizerStub.ExecuteCalled)
	require.True(t, skipNonDeletingStub.IsApplicableCalled)
	require.True(t, skipNonDeletingStub.ExecuteCalled)
	require.True(t, deleteManifestsStub.IsApplicableCalled)
	require.True(t, deleteManifestsStub.ExecuteCalled)
	require.True(t, removeFinalizerStub.IsApplicableCalled)
	require.True(t, removeFinalizerStub.ExecuteCalled)
}

func TestDeletionService_HandleDeletion_ErrorPropagation(t *testing.T) {
	t.Parallel()

	var executionOrder []string

	ensureFinalizerErrorStub := &UseCaseErrorStub{
		StubName:       "ensureFinalizer",
		ExecutionOrder: &executionOrder,
		ErrorMessage:   "ensureFinalizer failed",
	}
	skipNonDeletingStub := &UseCaseStub{UseCaseName: "skipNonDeleting", ExecutionOrder: &executionOrder}
	deleteManifestsStub := &UseCaseStub{UseCaseName: "deleteManifests", ExecutionOrder: &executionOrder}
	removeFinalizerStub := &UseCaseStub{UseCaseName: "removeFinalizer", ExecutionOrder: &executionOrder}

	service := deletion.NewService(
		ensureFinalizerErrorStub,
		skipNonDeletingStub,
		deleteManifestsStub,
		removeFinalizerStub,
	)
	mrm := &v1beta2.ModuleReleaseMeta{}

	for range 5 {
		err := service.HandleDeletion(context.Background(), mrm)
		require.Error(t, err)
		require.Contains(t, err.Error(), "ensureFinalizer failed")
	}

	expectedOrder := []string{
		"ensureFinalizer",
		"ensureFinalizer",
		"ensureFinalizer",
		"ensureFinalizer",
		"ensureFinalizer",
	}
	require.Equal(t, expectedOrder, executionOrder)
}

func TestDeletionService_HandleDeletion_IsApplicableError(t *testing.T) {
	t.Parallel()

	var executionOrder []string

	ensureFinalizerIsApplicableErrorStub := &UseCaseIsApplicableErrorStub{
		StubName:       "ensureFinalizer",
		ExecutionOrder: &executionOrder,
		ErrorMessage:   "IsApplicable failed",
	}
	skipNonDeletingStub := &UseCaseStub{UseCaseName: "skipNonDeleting", ExecutionOrder: &executionOrder}
	deleteManifestsStub := &UseCaseStub{UseCaseName: "deleteManifests", ExecutionOrder: &executionOrder}
	removeFinalizerStub := &UseCaseStub{UseCaseName: "removeFinalizer", ExecutionOrder: &executionOrder}

	service := deletion.NewService(
		ensureFinalizerIsApplicableErrorStub,
		skipNonDeletingStub,
		deleteManifestsStub,
		removeFinalizerStub,
	)
	mrm := &v1beta2.ModuleReleaseMeta{}

	err := service.HandleDeletion(context.Background(), mrm)
	require.Error(t, err)
	require.Contains(t, err.Error(), "IsApplicable failed")

	require.Empty(t, executionOrder)

	require.True(t, ensureFinalizerIsApplicableErrorStub.IsApplicableCalled)
	require.False(t, ensureFinalizerIsApplicableErrorStub.ExecuteCalled)
	require.False(t, skipNonDeletingStub.IsApplicableCalled)
	require.False(t, skipNonDeletingStub.ExecuteCalled)
	require.False(t, deleteManifestsStub.IsApplicableCalled)
	require.False(t, deleteManifestsStub.ExecuteCalled)
	require.False(t, removeFinalizerStub.IsApplicableCalled)
	require.False(t, removeFinalizerStub.ExecuteCalled)
}

// Stubs for the use cases to track execution order and calls

type UseCaseStub struct {
	IsApplicableCalled bool
	ExecuteCalled      bool
	ExecutionOrder     *[]string
	UseCaseName        string
}

func (stub *UseCaseStub) IsApplicable(_ context.Context, _ *v1beta2.ModuleReleaseMeta) (bool, error) {
	if stub.IsApplicableCalled {
		return false, nil
	}
	stub.IsApplicableCalled = true
	return true, nil
}

func (stub *UseCaseStub) Execute(_ context.Context, _ *v1beta2.ModuleReleaseMeta) error {
	stub.ExecuteCalled = true
	if stub.ExecutionOrder != nil {
		*stub.ExecutionOrder = append(*stub.ExecutionOrder, stub.UseCaseName)
	}
	return nil
}

type UseCaseErrorStub struct {
	IsApplicableCalled bool
	ExecuteCalled      bool
	ExecutionOrder     *[]string
	StubName           string
	ErrorMessage       string
}

func (stub *UseCaseErrorStub) IsApplicable(_ context.Context, _ *v1beta2.ModuleReleaseMeta) (bool, error) {
	stub.IsApplicableCalled = true
	return true, nil
}

func (stub *UseCaseErrorStub) Execute(_ context.Context, _ *v1beta2.ModuleReleaseMeta) error {
	stub.ExecuteCalled = true
	if stub.ExecutionOrder != nil {
		*stub.ExecutionOrder = append(*stub.ExecutionOrder, stub.StubName)
	}
	return errors.New(stub.ErrorMessage)
}

type UseCaseIsApplicableErrorStub struct {
	IsApplicableCalled bool
	ExecuteCalled      bool
	ExecutionOrder     *[]string
	StubName           string
	ErrorMessage       string
}

func (stub *UseCaseIsApplicableErrorStub) IsApplicable(_ context.Context, _ *v1beta2.ModuleReleaseMeta) (bool, error) {
	stub.IsApplicableCalled = true
	return false, errors.New(stub.ErrorMessage)
}

func (stub *UseCaseIsApplicableErrorStub) Execute(_ context.Context, _ *v1beta2.ModuleReleaseMeta) error {
	stub.ExecuteCalled = true
	if stub.ExecutionOrder != nil {
		*stub.ExecutionOrder = append(*stub.ExecutionOrder, stub.StubName)
	}
	return nil
}
