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

	skipNonMandatoryStub := &SkipNonMandatoryStub{StubName: "skipNonMandatory", ExecutionOrder: &executionOrder}
	ensureFinalizerStub := &EnsureFinalizerStub{StubName: "ensureFinalizer", ExecutionOrder: &executionOrder}
	skipNonDeletingStub := &SkipNonDeletingStub{StubName: "skipNonDeleting", ExecutionOrder: &executionOrder}
	deleteManifestsStub := &DeleteManifestsStub{StubName: "deleteManifests", ExecutionOrder: &executionOrder}
	removeFinalizerStub := &RemoveFinalizerStub{StubName: "removeFinalizer", ExecutionOrder: &executionOrder}

	service := deletion.NewService(
		skipNonMandatoryStub,
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
		"skipNonMandatory",
		"ensureFinalizer",
		"skipNonDeleting",
		"deleteManifests",
		"removeFinalizer",
	}
	require.Equal(t, expectedOrder, executionOrder)

	require.True(t, skipNonMandatoryStub.IsApplicableCalled)
	require.True(t, skipNonMandatoryStub.ExecuteCalled)
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

	skipNonMandatoryErrorStub := &SkipNonMandatoryErrorStub{
		StubName:       "skipNonMandatory",
		ExecutionOrder: &executionOrder,
	}
	ensureFinalizerStub := &EnsureFinalizerStub{StubName: "ensureFinalizer", ExecutionOrder: &executionOrder}
	skipNonDeletingStub := &SkipNonDeletingStub{StubName: "skipNonDeleting", ExecutionOrder: &executionOrder}
	deleteManifestsStub := &DeleteManifestsStub{StubName: "deleteManifests", ExecutionOrder: &executionOrder}
	removeFinalizerStub := &RemoveFinalizerStub{StubName: "removeFinalizer", ExecutionOrder: &executionOrder}

	service := deletion.NewService(
		skipNonMandatoryErrorStub,
		ensureFinalizerStub,
		skipNonDeletingStub,
		deleteManifestsStub,
		removeFinalizerStub,
	)
	mrm := &v1beta2.ModuleReleaseMeta{}

	for range 5 {
		err := service.HandleDeletion(context.Background(), mrm)
		require.Error(t, err)
		require.Contains(t, err.Error(), "skipNonMandatory failed")
	}

	expectedOrder := []string{
		"skipNonMandatory",
		"skipNonMandatory",
		"skipNonMandatory",
		"skipNonMandatory",
		"skipNonMandatory",
	}
	require.Equal(t, expectedOrder, executionOrder)
}

func TestDeletionService_HandleDeletion_IsApplicableError(t *testing.T) {
	t.Parallel()

	var executionOrder []string

	skipNonMandatoryIsApplicableErrorStub := &SkipNonMandatoryIsApplicableErrorStub{
		StubName:       "skipNonMandatory",
		ExecutionOrder: &executionOrder,
	}
	ensureFinalizerStub := &EnsureFinalizerStub{StubName: "ensureFinalizer", ExecutionOrder: &executionOrder}
	skipNonDeletingStub := &SkipNonDeletingStub{StubName: "skipNonDeleting", ExecutionOrder: &executionOrder}
	deleteManifestsStub := &DeleteManifestsStub{StubName: "deleteManifests", ExecutionOrder: &executionOrder}
	removeFinalizerStub := &RemoveFinalizerStub{StubName: "removeFinalizer", ExecutionOrder: &executionOrder}

	service := deletion.NewService(
		skipNonMandatoryIsApplicableErrorStub,
		ensureFinalizerStub,
		skipNonDeletingStub,
		deleteManifestsStub,
		removeFinalizerStub,
	)
	mrm := &v1beta2.ModuleReleaseMeta{}

	err := service.HandleDeletion(context.Background(), mrm)
	require.Error(t, err)
	require.Contains(t, err.Error(), "IsApplicable failed")

	require.Empty(t, executionOrder)

	require.True(t, skipNonMandatoryIsApplicableErrorStub.IsApplicableCalled)
	require.False(t, skipNonMandatoryIsApplicableErrorStub.ExecuteCalled)
	require.False(t, ensureFinalizerStub.IsApplicableCalled)
	require.False(t, ensureFinalizerStub.ExecuteCalled)
	require.False(t, skipNonDeletingStub.IsApplicableCalled)
	require.False(t, skipNonDeletingStub.ExecuteCalled)
	require.False(t, deleteManifestsStub.IsApplicableCalled)
	require.False(t, deleteManifestsStub.ExecuteCalled)
	require.False(t, removeFinalizerStub.IsApplicableCalled)
	require.False(t, removeFinalizerStub.ExecuteCalled)
}

// Stubs for the use cases to track execution order and calls

type SkipNonMandatoryStub struct {
	IsApplicableCalled bool
	ExecuteCalled      bool
	ExecutionOrder     *[]string
	StubName           string
}

func (stub *SkipNonMandatoryStub) IsApplicable(_ context.Context, _ *v1beta2.ModuleReleaseMeta) (bool, error) {
	if stub.IsApplicableCalled {
		return false, nil
	}
	stub.IsApplicableCalled = true
	return true, nil
}

func (stub *SkipNonMandatoryStub) Execute(_ context.Context, _ *v1beta2.ModuleReleaseMeta) error {
	stub.ExecuteCalled = true
	if stub.ExecutionOrder != nil {
		*stub.ExecutionOrder = append(*stub.ExecutionOrder, stub.StubName)
	}
	return nil
}

type EnsureFinalizerStub struct {
	IsApplicableCalled bool
	ExecuteCalled      bool
	ExecutionOrder     *[]string
	StubName           string
}

func (stub *EnsureFinalizerStub) IsApplicable(_ context.Context, _ *v1beta2.ModuleReleaseMeta) (bool, error) {
	if stub.IsApplicableCalled {
		return false, nil
	}
	stub.IsApplicableCalled = true
	return true, nil
}

func (stub *EnsureFinalizerStub) Execute(_ context.Context, _ *v1beta2.ModuleReleaseMeta) error {
	stub.ExecuteCalled = true
	if stub.ExecutionOrder != nil {
		*stub.ExecutionOrder = append(*stub.ExecutionOrder, stub.StubName)
	}
	return nil
}

type SkipNonDeletingStub struct {
	IsApplicableCalled bool
	ExecuteCalled      bool
	ExecutionOrder     *[]string
	StubName           string
}

func (stub *SkipNonDeletingStub) IsApplicable(_ context.Context, _ *v1beta2.ModuleReleaseMeta) (bool, error) {
	if stub.IsApplicableCalled {
		return false, nil
	}
	stub.IsApplicableCalled = true
	return true, nil
}

func (stub *SkipNonDeletingStub) Execute(_ context.Context, _ *v1beta2.ModuleReleaseMeta) error {
	stub.ExecuteCalled = true
	if stub.ExecutionOrder != nil {
		*stub.ExecutionOrder = append(*stub.ExecutionOrder, stub.StubName)
	}
	return nil
}

type DeleteManifestsStub struct {
	IsApplicableCalled bool
	ExecuteCalled      bool
	ExecutionOrder     *[]string
	StubName           string
}

func (stub *DeleteManifestsStub) IsApplicable(_ context.Context, _ *v1beta2.ModuleReleaseMeta) (bool, error) {
	if stub.IsApplicableCalled {
		return false, nil
	}
	stub.IsApplicableCalled = true
	return true, nil
}

func (stub *DeleteManifestsStub) Execute(_ context.Context, _ *v1beta2.ModuleReleaseMeta) error {
	stub.ExecuteCalled = true
	if stub.ExecutionOrder != nil {
		*stub.ExecutionOrder = append(*stub.ExecutionOrder, stub.StubName)
	}
	return nil
}

type RemoveFinalizerStub struct {
	IsApplicableCalled bool
	ExecuteCalled      bool
	ExecutionOrder     *[]string
	StubName           string
}

func (stub *RemoveFinalizerStub) IsApplicable(_ context.Context, _ *v1beta2.ModuleReleaseMeta) (bool, error) {
	if stub.IsApplicableCalled {
		return false, nil
	}
	stub.IsApplicableCalled = true
	return true, nil
}

func (stub *RemoveFinalizerStub) Execute(_ context.Context, _ *v1beta2.ModuleReleaseMeta) error {
	stub.ExecuteCalled = true
	if stub.ExecutionOrder != nil {
		*stub.ExecutionOrder = append(*stub.ExecutionOrder, stub.StubName)
	}
	return nil
}

type SkipNonMandatoryErrorStub struct {
	IsApplicableCalled bool
	ExecuteCalled      bool
	ExecutionOrder     *[]string
	StubName           string
}

func (stub *SkipNonMandatoryErrorStub) IsApplicable(_ context.Context, _ *v1beta2.ModuleReleaseMeta) (bool, error) {
	stub.IsApplicableCalled = true
	return true, nil
}

func (stub *SkipNonMandatoryErrorStub) Execute(_ context.Context, _ *v1beta2.ModuleReleaseMeta) error {
	stub.ExecuteCalled = true
	if stub.ExecutionOrder != nil {
		*stub.ExecutionOrder = append(*stub.ExecutionOrder, stub.StubName)
	}
	return errors.New("skipNonMandatory failed")
}

type SkipNonMandatoryIsApplicableErrorStub struct {
	IsApplicableCalled bool
	ExecuteCalled      bool
	ExecutionOrder     *[]string
	StubName           string
}

func (stub *SkipNonMandatoryIsApplicableErrorStub) IsApplicable(_ context.Context,
	_ *v1beta2.ModuleReleaseMeta,
) (bool, error) {
	stub.IsApplicableCalled = true
	return false, errors.New("IsApplicable failed")
}

func (stub *SkipNonMandatoryIsApplicableErrorStub) Execute(_ context.Context, _ *v1beta2.ModuleReleaseMeta) error {
	stub.ExecuteCalled = true
	if stub.ExecutionOrder != nil {
		*stub.ExecutionOrder = append(*stub.ExecutionOrder, stub.StubName)
	}
	return nil
}
