package usecases_test

import (
	"context"
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/deletion/usecases"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestExecute_Succeeds(t *testing.T) {
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	kymaStatusRepo := &kymaStatusRepoStub{}

	uc := usecases.NewSetSkrKymaStateDeleting(kymaStatusRepo, nil)

	result := uc.Execute(t.Context(), kyma)

	require.NoError(t, result.Err)
	assert.Equal(t, usecase.SetSkrKymaStateDeleting, result.UseCase)
	assert.True(t, kymaStatusRepo.called)
	assert.Equal(t, kyma.GetNamespacedName(), kymaStatusRepo.namespacedName)
}

func TestExecute_Fails(t *testing.T) {
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	kymaStatusRepo := &kymaStatusRepoStub{
		err: assert.AnError,
	}

	uc := usecases.NewSetSkrKymaStateDeleting(kymaStatusRepo, nil)

	result := uc.Execute(t.Context(), kyma)

	require.ErrorIs(t, result.Err, assert.AnError)
	assert.Equal(t, usecase.SetSkrKymaStateDeleting, result.UseCase)
	assert.True(t, kymaStatusRepo.called)
	assert.Equal(t, kyma.GetNamespacedName(), kymaStatusRepo.namespacedName)
}

type kymaStatusRepoStub struct {
	usecases.KymaStatusRepo

	called bool

	namespacedName types.NamespacedName

	err error
}

func (r *kymaStatusRepoStub) SetStateDeleting(_ context.Context, namespacedName types.NamespacedName) error {
	r.called = true
	r.namespacedName = namespacedName
	return r.err
}
