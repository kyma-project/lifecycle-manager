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
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
	kymadeletionsvc "github.com/kyma-project/lifecycle-manager/internal/service/kyma/deletion"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func Test_NewService_ReturnsError_WhenUseCasesAreOutOfOrder(t *testing.T) {
	useCases := setupUseCases()
	// swap two use cases to simulate out-of-order scenario
	useCases[2], useCases[3] = useCases[3], useCases[2]

	svc, err := kymadeletionsvc.NewService(
		useCases[0],
		useCases[1],
		useCases[2],
		useCases[3],
		useCases[4],
		useCases[5],
		useCases[6],
		useCases[7],
		useCases[8],
		useCases[9],
		useCases[10],
	)

	require.Nil(t, svc)
	require.ErrorIs(t, err, kymadeletionsvc.ErrUseCasesOutOfOrder)
	require.Contains(t, err.Error(), "expected use case DeleteSkrKyma at position 2 but found DeleteCertificateSetup")
	require.Contains(t, err.Error(), "expected use case DeleteCertificateSetup at position 3 but found DeleteSkrKyma")
}

func Test_NewService_ReturnsError_WhenSameUseCaseTwice(t *testing.T) {
	useCases := setupUseCases()
	// duplicate a use case to simulate out-of-order scenario
	useCases[2] = useCases[1]

	svc, err := kymadeletionsvc.NewService(
		useCases[0],
		useCases[1],
		useCases[2],
		useCases[3],
		useCases[4],
		useCases[5],
		useCases[6],
		useCases[7],
		useCases[8],
		useCases[9],
		useCases[10],
	)

	require.Nil(t, svc)
	require.ErrorIs(t, err, kymadeletionsvc.ErrUseCasesOutOfOrder)
}

func Test_Delete_ReturnsError_WhenIsApplicableReturnsError(t *testing.T) {
	kyma := &v1beta2.Kyma{}
	useCases := setupUseCases()
	uc1 := useCases[0]
	uc1.err = assert.AnError
	uc1.isApplicable = true

	svc, err := kymadeletionsvc.NewService(
		useCases[0],
		useCases[1],
		useCases[2],
		useCases[3],
		useCases[4],
		useCases[5],
		useCases[6],
		useCases[7],
		useCases[8],
		useCases[9],
		useCases[10],
	)

	result := svc.Delete(t.Context(), kyma)

	require.NoError(t, err)
	require.ErrorIs(t, result.Err, assert.AnError)
	assert.Equal(t, uc1.Name(), result.UseCase)
	assert.True(t, uc1.isApplicableCalled)
	assert.False(t, uc1.executeCalled)
	assert.Equal(t, kyma, uc1.receivedKyma)
}

func Test_Delete_ReturnsEarly_WhenIsApplicableReturnsError(t *testing.T) {
	kyma := &v1beta2.Kyma{}
	useCases := setupUseCases()
	uc1 := useCases[0]
	uc2 := useCases[1]
	uc3 := useCases[2]

	uc2.isApplicable = true
	uc2.err = assert.AnError

	svc, err := kymadeletionsvc.NewService(
		useCases[0],
		useCases[1],
		useCases[2],
		useCases[3],
		useCases[4],
		useCases[5],
		useCases[6],
		useCases[7],
		useCases[8],
		useCases[9],
		useCases[10],
	)

	result := svc.Delete(t.Context(), kyma)

	require.NoError(t, err)
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
	useCases := setupUseCases()
	uc1 := useCases[0]
	uc2 := useCases[1]
	uc3 := useCases[2]

	uc2.isApplicable = true
	uc3.isApplicable = true

	svc, err := kymadeletionsvc.NewService(
		useCases[0],
		useCases[1],
		useCases[2],
		useCases[3],
		useCases[4],
		useCases[5],
		useCases[6],
		useCases[7],
		useCases[8],
		useCases[9],
		useCases[10],
	)

	result := svc.Delete(t.Context(), kyma)

	require.NoError(t, err)
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
	useCases := setupUseCases()
	uc1 := useCases[0]
	uc2 := useCases[1]
	uc3 := useCases[2]
	uc4 := useCases[3]
	uc5 := useCases[4]
	uc6 := useCases[5]
	uc7 := useCases[6]
	uc8 := useCases[7]
	uc9 := useCases[8]
	uc10 := useCases[9]
	uc11 := useCases[10]

	svc, err := kymadeletionsvc.NewService(
		useCases[0],
		useCases[1],
		useCases[2],
		useCases[3],
		useCases[4],
		useCases[5],
		useCases[6],
		useCases[7],
		useCases[8],
		useCases[9],
		useCases[10],
	)

	rslt := svc.Delete(t.Context(), kyma)

	require.NoError(t, err)
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
	useCases := setupUseCases()

	// recording order of the first three should be sufficient
	recordedOrder := []string{}
	uc1 := &orderRecordingUseCaseStub{recorder: &recordedOrder, name: usecase.SetKcpKymaStateDeleting}
	uc2 := &orderRecordingUseCaseStub{recorder: &recordedOrder, name: usecase.SetSkrKymaStateDeleting}
	uc3 := &orderRecordingUseCaseStub{recorder: &recordedOrder, name: usecase.DeleteSkrKyma}

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

	svc, _ := kymadeletionsvc.NewService(
		uc1,
		uc2,
		uc3,
		useCases[3],
		useCases[4],
		useCases[5],
		useCases[6],
		useCases[7],
		useCases[8],
		useCases[9],
		useCases[10],
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

func setupUseCases() []*useCaseStub {
	return []*useCaseStub{
		{isApplicable: false, err: nil, name: usecase.SetKcpKymaStateDeleting},
		{isApplicable: false, err: nil, name: usecase.SetSkrKymaStateDeleting},
		{isApplicable: false, err: nil, name: usecase.DeleteSkrKyma},
		{isApplicable: false, err: nil, name: usecase.DeleteWatcherCertificateSetup},
		{isApplicable: false, err: nil, name: usecase.DeleteSkrWebhookResources},
		{isApplicable: false, err: nil, name: usecase.DeleteSkrModuleTemplateCrd},
		{isApplicable: false, err: nil, name: usecase.DeleteSkrModuleReleaseMetaCrd},
		{isApplicable: false, err: nil, name: usecase.DeleteSkrKymaCrd},
		{isApplicable: false, err: nil, name: usecase.DeleteManifests},
		{isApplicable: false, err: nil, name: usecase.DeleteMetrics},
		{isApplicable: false, err: nil, name: usecase.DropKymaFinalizer},
	}
}
