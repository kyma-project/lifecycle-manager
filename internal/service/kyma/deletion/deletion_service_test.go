package deletion_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/errors/kyma/deletion"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	kymadeletionsvc "github.com/kyma-project/lifecycle-manager/internal/service/kyma/deletion"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func Test_Delete_ReturnsError_WhenIsApplicableReturnsError(t *testing.T) {
	kyma := &v1beta2.Kyma{}
	uc1 := &useCaseStub{isApplicable: true, err: assert.AnError}

	svc := kymadeletionsvc.NewService(
		uc1,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
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

	svc := kymadeletionsvc.NewService(
		uc1,
		uc2,
		uc3,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
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

	svc := kymadeletionsvc.NewService(
		uc1,
		uc2,
		uc3,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
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
	uc4 := &useCaseStub{isApplicable: false, err: nil}
	uc5 := &useCaseStub{isApplicable: false, err: nil}
	uc6 := &useCaseStub{isApplicable: false, err: nil}
	uc7 := &useCaseStub{isApplicable: false, err: nil}
	uc8 := &useCaseStub{isApplicable: false, err: nil}
	uc9 := &useCaseStub{isApplicable: false, err: nil}
	uc10 := &useCaseStub{isApplicable: false, err: nil}
	uc11 := &useCaseStub{isApplicable: false, err: nil}

	svc := kymadeletionsvc.NewService(
		uc1,
		uc2,
		uc3,
		uc4,
		uc5,
		uc6,
		uc7,
		uc8,
		uc9,
		uc10,
		uc11,
	)

	rslt := svc.Delete(t.Context(), kyma)

	require.ErrorIs(t, rslt.Err, deletion.ErrNoUseCaseApplicable)
	assert.Equal(t, result.UseCase(""), rslt.UseCase)
	assert.True(t, uc1.isApplicableCalled)
	assert.False(t, uc1.executeCalled)
	assert.True(t, uc2.isApplicableCalled)
	assert.False(t, uc2.executeCalled)
	assert.True(t, uc3.isApplicableCalled)
	assert.False(t, uc3.executeCalled)
	assert.True(t, uc4.isApplicableCalled)
	assert.False(t, uc4.executeCalled)
	assert.True(t, uc5.isApplicableCalled)
	assert.False(t, uc5.executeCalled)
	assert.True(t, uc6.isApplicableCalled)
	assert.False(t, uc6.executeCalled)
	assert.True(t, uc7.isApplicableCalled)
	assert.False(t, uc7.executeCalled)
	assert.True(t, uc8.isApplicableCalled)
	assert.False(t, uc8.executeCalled)
	assert.True(t, uc9.isApplicableCalled)
	assert.False(t, uc9.executeCalled)
	assert.True(t, uc10.isApplicableCalled)
	assert.False(t, uc10.executeCalled)
	assert.True(t, uc11.isApplicableCalled)
	assert.False(t, uc11.executeCalled)
	assert.Equal(t, kyma, uc1.receivedKyma)
	assert.Equal(t, kyma, uc2.receivedKyma)
	assert.Equal(t, kyma, uc3.receivedKyma)
	assert.Equal(t, kyma, uc4.receivedKyma)
	assert.Equal(t, kyma, uc5.receivedKyma)
	assert.Equal(t, kyma, uc6.receivedKyma)
	assert.Equal(t, kyma, uc7.receivedKyma)
}

func Test_Delete_ExecutesCorrectOrderOfUseCases(t *testing.T) {
	kyma := &v1beta2.Kyma{}

	recordedOrder := []string{}
	uc1 := &orderRecordingUseCaseStub{recorder: &recordedOrder}
	uc2 := &orderRecordingUseCaseStub{recorder: &recordedOrder}
	uc3 := &orderRecordingUseCaseStub{recorder: &recordedOrder}

	executionOrder := []string{
		fmt.Sprintf("%s-%s", uc1.Name(), "isApplicable"),
		fmt.Sprintf("%s-%s", uc1.Name(), "execute"),
		fmt.Sprintf("%s-%s", uc1.Name(), "isApplicable"),
		fmt.Sprintf("%s-%s", uc2.Name(), "isApplicable"),
		fmt.Sprintf("%s-%s", uc2.Name(), "execute"),
		fmt.Sprintf("%s-%s", uc1.Name(), "isApplicable"),
		fmt.Sprintf("%s-%s", uc2.Name(), "isApplicable"),
		fmt.Sprintf("%s-%s", uc3.Name(), "isApplicable"),
		fmt.Sprintf("%s-%s", uc3.Name(), "execute"),
	}

	svc := kymadeletionsvc.NewService(
		uc1,
		uc2,
		uc3,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	_ = svc.Delete(t.Context(), kyma)
	_ = svc.Delete(t.Context(), kyma)
	_ = svc.Delete(t.Context(), kyma)

	require.Equal(t, executionOrder, recordedOrder)
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

type orderRecordingUseCaseStub struct {
	name        result.UseCase
	appliedOnce bool
	recorder    *[]string
}

func (u *orderRecordingUseCaseStub) IsApplicable(_ context.Context, _ *v1beta2.Kyma) (bool, error) {
	u.record("isApplicable")

	if !u.appliedOnce {
		u.appliedOnce = true
		return true, nil
	}

	return false, nil
}

func (u *orderRecordingUseCaseStub) Execute(_ context.Context, _ *v1beta2.Kyma) result.Result {
	u.record("execute")
	return result.Result{
		UseCase: u.Name(),
		Err:     nil,
	}
}

func (u *orderRecordingUseCaseStub) Name() result.UseCase {
	if u.name == "" {
		u.name = result.UseCase(random.Name())
	}

	return u.name
}

func (u *orderRecordingUseCaseStub) record(phase string) {
	*u.recorder = append(*u.recorder, fmt.Sprintf("%s-%s", u.Name(), phase))
}
