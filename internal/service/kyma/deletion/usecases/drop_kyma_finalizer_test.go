package usecases_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/deletion/usecases"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func Test_DropKymaFinalizer_IsApplicable_KymaFinalizerExists(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:       random.Name(),
			Namespace:  random.Name(),
			Finalizers: []string{random.Name(), random.Name(), shared.KymaFinalizer},
		},
	}

	kymaRepo := &kymaRepoStub{}

	uc := usecases.NewDropKymaFinalizer(kymaRepo)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.NoError(t, err)
	assert.True(t, applicable)
}

func Test_DropKymaFinalizer_IsApplicable_NoFinalizers(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:       random.Name(),
			Namespace:  random.Name(),
			Finalizers: []string{},
		},
	}

	kymaRepo := &kymaRepoStub{}

	uc := usecases.NewDropKymaFinalizer(kymaRepo)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.NoError(t, err)
	assert.False(t, applicable)
}

func Test_DropKymaFinalizer_IsApplicable_FinalizersNil(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:       random.Name(),
			Namespace:  random.Name(),
			Finalizers: nil,
		},
	}

	kymaRepo := &kymaRepoStub{}

	uc := usecases.NewDropKymaFinalizer(kymaRepo)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.NoError(t, err)
	assert.False(t, applicable)
}

func Test_DropKymaFinalizer_Execute_Success(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	kymaRepo := &kymaRepoStub{}

	uc := usecases.NewDropKymaFinalizer(kymaRepo)

	res := uc.Execute(t.Context(), kcpKyma)

	require.NoError(t, res.Err)
	assert.Equal(t, usecase.DropKymaFinalizer, res.UseCase)
	assert.True(t, kymaRepo.dropFinalizersCalled)
	assert.Equal(t, kcpKyma.GetName(), kymaRepo.kymaName)
}

func Test_DropKymaFinalizer_Execute_Failure(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	kymaRepo := &kymaRepoStub{
		dropFinalizersErr: assert.AnError,
	}

	uc := usecases.NewDropKymaFinalizer(kymaRepo)

	res := uc.Execute(t.Context(), kcpKyma)

	require.ErrorIs(t, res.Err, assert.AnError)
	assert.Equal(t, usecase.DropKymaFinalizer, res.UseCase)
	assert.True(t, kymaRepo.dropFinalizersCalled)
	assert.Equal(t, kcpKyma.GetName(), kymaRepo.kymaName)
}

type kymaRepoStub struct {
	usecases.KymaRepo

	dropFinalizersCalled bool
	kymaName             string
	dropFinalizersErr    error
}

func (r *kymaRepoStub) DropKymaFinalizer(_ context.Context, kymaName string) error {
	r.dropFinalizersCalled = true
	r.kymaName = kymaName
	return r.dropFinalizersErr
}
