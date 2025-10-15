package deletion_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule/deletion"
	"github.com/stretchr/testify/require"
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

	for i := 0; i < 5; i++ {
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

	initialLen := len(executionOrder)
	err := service.HandleDeletion(context.Background(), mrm)
	require.NoError(t, err)
	require.Equal(t, initialLen, len(executionOrder))

	require.True(t, skipNonMandatoryStub.ShouldExecuteCalled)
	require.True(t, skipNonMandatoryStub.ExecuteCalled)
	require.True(t, ensureFinalizerStub.ShouldExecuteCalled)
	require.True(t, ensureFinalizerStub.ExecuteCalled)
	require.True(t, skipNonDeletingStub.ShouldExecuteCalled)
	require.True(t, skipNonDeletingStub.ExecuteCalled)
	require.True(t, deleteManifestsStub.ShouldExecuteCalled)
	require.True(t, deleteManifestsStub.ExecuteCalled)
	require.True(t, removeFinalizerStub.ShouldExecuteCalled)
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

	for i := 0; i < 5; i++ {
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

func TestDeletionService_HandleDeletion_ShouldExecuteError(t *testing.T) {
	t.Parallel()

	var executionOrder []string

	skipNonMandatoryShouldExecuteErrorStub := &SkipNonMandatoryShouldExecuteErrorStub{
		StubName:       "skipNonMandatory",
		ExecutionOrder: &executionOrder,
	}
	ensureFinalizerStub := &EnsureFinalizerStub{StubName: "ensureFinalizer", ExecutionOrder: &executionOrder}
	skipNonDeletingStub := &SkipNonDeletingStub{StubName: "skipNonDeleting", ExecutionOrder: &executionOrder}
	deleteManifestsStub := &DeleteManifestsStub{StubName: "deleteManifests", ExecutionOrder: &executionOrder}
	removeFinalizerStub := &RemoveFinalizerStub{StubName: "removeFinalizer", ExecutionOrder: &executionOrder}

	service := deletion.NewService(
		skipNonMandatoryShouldExecuteErrorStub,
		ensureFinalizerStub,
		skipNonDeletingStub,
		deleteManifestsStub,
		removeFinalizerStub,
	)
	mrm := &v1beta2.ModuleReleaseMeta{}

	err := service.HandleDeletion(context.Background(), mrm)
	require.Error(t, err)
	require.Contains(t, err.Error(), "shouldExecute failed")

	require.Empty(t, executionOrder)

	require.True(t, skipNonMandatoryShouldExecuteErrorStub.ShouldExecuteCalled)
	require.False(t, skipNonMandatoryShouldExecuteErrorStub.ExecuteCalled)
	require.False(t, ensureFinalizerStub.ShouldExecuteCalled)
	require.False(t, ensureFinalizerStub.ExecuteCalled)
	require.False(t, skipNonDeletingStub.ShouldExecuteCalled)
	require.False(t, skipNonDeletingStub.ExecuteCalled)
	require.False(t, deleteManifestsStub.ShouldExecuteCalled)
	require.False(t, deleteManifestsStub.ExecuteCalled)
	require.False(t, removeFinalizerStub.ShouldExecuteCalled)
	require.False(t, removeFinalizerStub.ExecuteCalled)
}

// Stubs for the use cases to track execution order and calls

type SkipNonMandatoryStub struct {
	ShouldExecuteCalled bool
	ExecuteCalled       bool
	ExecutionOrder      *[]string
	StubName            string
}

func (stub *SkipNonMandatoryStub) ShouldExecute(_ context.Context, _ *v1beta2.ModuleReleaseMeta) (bool, error) {
	if stub.ShouldExecuteCalled {
		return false, nil
	}
	stub.ShouldExecuteCalled = true
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
	ShouldExecuteCalled bool
	ExecuteCalled       bool
	ExecutionOrder      *[]string
	StubName            string
}

func (stub *EnsureFinalizerStub) ShouldExecute(_ context.Context, _ *v1beta2.ModuleReleaseMeta) (bool, error) {
	if stub.ShouldExecuteCalled {
		return false, nil
	}
	stub.ShouldExecuteCalled = true
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
	ShouldExecuteCalled bool
	ExecuteCalled       bool
	ExecutionOrder      *[]string
	StubName            string
}

func (stub *SkipNonDeletingStub) ShouldExecute(_ context.Context, _ *v1beta2.ModuleReleaseMeta) (bool, error) {
	if stub.ShouldExecuteCalled {
		return false, nil
	}
	stub.ShouldExecuteCalled = true
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
	ShouldExecuteCalled bool
	ExecuteCalled       bool
	ExecutionOrder      *[]string
	StubName            string
}

func (stub *DeleteManifestsStub) ShouldExecute(_ context.Context, _ *v1beta2.ModuleReleaseMeta) (bool, error) {
	if stub.ShouldExecuteCalled {
		return false, nil
	}
	stub.ShouldExecuteCalled = true
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
	ShouldExecuteCalled bool
	ExecuteCalled       bool
	ExecutionOrder      *[]string
	StubName            string
}

func (stub *RemoveFinalizerStub) ShouldExecute(_ context.Context, _ *v1beta2.ModuleReleaseMeta) (bool, error) {
	if stub.ShouldExecuteCalled {
		return false, nil
	}
	stub.ShouldExecuteCalled = true
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
	ShouldExecuteCalled bool
	ExecuteCalled       bool
	ExecutionOrder      *[]string
	StubName            string
}

func (stub *SkipNonMandatoryErrorStub) ShouldExecute(_ context.Context, _ *v1beta2.ModuleReleaseMeta) (bool, error) {
	stub.ShouldExecuteCalled = true
	return true, nil
}

func (stub *SkipNonMandatoryErrorStub) Execute(_ context.Context, _ *v1beta2.ModuleReleaseMeta) error {
	stub.ExecuteCalled = true
	if stub.ExecutionOrder != nil {
		*stub.ExecutionOrder = append(*stub.ExecutionOrder, stub.StubName)
	}
	return errors.New("skipNonMandatory failed")
}

type SkipNonMandatoryShouldExecuteErrorStub struct {
	ShouldExecuteCalled bool
	ExecuteCalled       bool
	ExecutionOrder      *[]string
	StubName            string
}

func (stub *SkipNonMandatoryShouldExecuteErrorStub) ShouldExecute(_ context.Context,
	_ *v1beta2.ModuleReleaseMeta,
) (bool, error) {
	stub.ShouldExecuteCalled = true
	return false, errors.New("shouldExecute failed")
}

func (stub *SkipNonMandatoryShouldExecuteErrorStub) Execute(_ context.Context, _ *v1beta2.ModuleReleaseMeta) error {
	stub.ExecuteCalled = true
	if stub.ExecutionOrder != nil {
		*stub.ExecutionOrder = append(*stub.ExecutionOrder, stub.StubName)
	}
	return nil
}
