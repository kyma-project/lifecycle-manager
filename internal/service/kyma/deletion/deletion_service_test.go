package deletion_test

import (
	"context"
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/deletion"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Delete_ReturnsError_WhenIsApplicableReturnsError(t *testing.T) {
	kyma := &v1beta2.Kyma{}
	uc1 := &useCaseStub{isApplicable: true, err: assert.AnError}

	svc := deletion.NewService(
		uc1,
		nil,
		nil,
	)

	result := svc.Delete(t.Context(), kyma)

	require.ErrorIs(t, result.Err, assert.AnError)
	assert.Equal(t, uc1.Name(), result.UseCase)
	assert.True(t, uc1.isApplicableCalled)
	assert.False(t, uc1.executeCalled)
	assert.Equal(t, kyma, uc1.receivedKyma)
}

func Test_Delete_ReturnsEarly_WhenIsApplicableReturnsError(t *testing.T) {
	kyma := &v1beta2.Kyma{}
	uc1 := &useCaseStub{isApplicable: false, err: nil}
	uc2 := &useCaseStub{isApplicable: true, err: assert.AnError}
	uc3 := &useCaseStub{isApplicable: false, err: nil}

	svc := deletion.NewService(
		uc1,
		uc2,
		uc3,
	)

	result := svc.Delete(t.Context(), kyma)

	require.ErrorIs(t, result.Err, assert.AnError)
	assert.Equal(t, uc2.Name(), result.UseCase)
	assert.True(t, uc1.isApplicableCalled)
	assert.False(t, uc1.executeCalled)
	assert.True(t, uc2.isApplicableCalled)
	assert.False(t, uc2.executeCalled)
	assert.False(t, uc3.isApplicableCalled)
	assert.False(t, uc3.executeCalled)
	assert.Equal(t, kyma, uc1.receivedKyma)
	assert.Equal(t, kyma, uc2.receivedKyma)
}

func Test_Delete_ExecutesOnlyFirstApplicableUseCase(t *testing.T) {
	kyma := &v1beta2.Kyma{}
	uc1 := &useCaseStub{isApplicable: false, err: nil}
	uc2 := &useCaseStub{isApplicable: true, err: nil}
	uc3 := &useCaseStub{isApplicable: true, err: nil}

	svc := deletion.NewService(
		uc1,
		uc2,
		uc3,
	)

	result := svc.Delete(t.Context(), kyma)

	require.NoError(t, result.Err)
	assert.Equal(t, uc2.Name(), result.UseCase)
	assert.True(t, uc1.isApplicableCalled)
	assert.False(t, uc1.executeCalled)
	assert.True(t, uc2.isApplicableCalled)
	assert.True(t, uc2.executeCalled)
	assert.False(t, uc3.isApplicableCalled)
	assert.False(t, uc3.executeCalled)
	assert.Equal(t, kyma, uc1.receivedKyma)
	assert.Equal(t, kyma, uc2.receivedKyma)
}

func Test_Delete_Fallthrough_WhenNoUseCaseIsApplicable(t *testing.T) {
	kyma := &v1beta2.Kyma{}
	uc1 := &useCaseStub{isApplicable: false, err: nil}
	uc2 := &useCaseStub{isApplicable: false, err: nil}
	uc3 := &useCaseStub{isApplicable: false, err: nil}

	svc := deletion.NewService(
		uc1,
		uc2,
		uc3,
	)

	result := svc.Delete(t.Context(), kyma)

	require.ErrorIs(t, result.Err, deletion.ErrNoUseCaseApplicable)
	assert.Equal(t, usecase.ProcessKymaDeletion, result.UseCase)
	assert.True(t, uc1.isApplicableCalled)
	assert.False(t, uc1.executeCalled)
	assert.True(t, uc2.isApplicableCalled)
	assert.False(t, uc2.executeCalled)
	assert.True(t, uc3.isApplicableCalled)
	assert.False(t, uc3.executeCalled)
	assert.Equal(t, kyma, uc1.receivedKyma)
	assert.Equal(t, kyma, uc2.receivedKyma)
	assert.Equal(t, kyma, uc3.receivedKyma)
}

type useCaseStub struct {
	receivedKyma *v1beta2.Kyma
	name         result.UseCase

	isApplicableCalled bool
	executeCalled      bool

	isApplicable bool
	err          error
}

func (u *useCaseStub) IsApplicable(_ context.Context, kyma *v1beta2.Kyma) (bool, error) {
	u.receivedKyma = kyma
	u.isApplicableCalled = true
	return u.isApplicable, u.err
}

func (u *useCaseStub) Execute(_ context.Context, kyma *v1beta2.Kyma) result.Result {
	u.receivedKyma = kyma
	u.executeCalled = true

	return result.Result{
		UseCase: u.Name(),
		Err:     u.err,
	}
}

func (u *useCaseStub) Name() result.UseCase {
	if u.name == "" {
		u.name = result.UseCase(random.Name())
	}

	return u.name
}
