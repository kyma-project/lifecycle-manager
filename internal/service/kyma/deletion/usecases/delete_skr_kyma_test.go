package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/deletion/usecases"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestIsApplicable_KcpKymaNotDeleting(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	uc := usecases.NewDeleteSkrKyma(nil, nil)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.NoError(t, err)
	assert.False(t, applicable)
}

func TestIsApplicable_SecretDoesNotExist(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:              random.Name(),
			Namespace:         random.Name(),
			DeletionTimestamp: &apimetav1.Time{Time: time.Now()},
		},
	}

	skrAccessSecretRepo := &skrAccessSecretRepoStub{
		exists: false,
	}

	uc := usecases.NewDeleteSkrKyma(nil, skrAccessSecretRepo)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.NoError(t, err)
	assert.False(t, applicable)
	assert.True(t, skrAccessSecretRepo.called)
	assert.Equal(t, kcpKyma.GetName(), skrAccessSecretRepo.kymaName)
}

func TestIsApplicable_SecretRepoReturnsError(t *testing.T) {
	time := apimetav1.NewTime(time.Now())
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:              random.Name(),
			Namespace:         random.Name(),
			DeletionTimestamp: &time,
		},
	}

	skrAccessSecretRepo := &skrAccessSecretRepoStub{
		err: assert.AnError,
	}

	uc := usecases.NewDeleteSkrKyma(nil, skrAccessSecretRepo)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.ErrorIs(t, err, assert.AnError)
	assert.False(t, applicable)
	assert.True(t, skrAccessSecretRepo.called)
	assert.Equal(t, kcpKyma.GetName(), skrAccessSecretRepo.kymaName)
}

func TestIsApplicable_KymaAlreadyDeleting(t *testing.T) {
	time := apimetav1.NewTime(time.Now())
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:              random.Name(),
			Namespace:         random.Name(),
			DeletionTimestamp: &time,
		},
	}

	skrKymaRepo := &skrKymaRepoStub{
		isDeleting: true,
	}

	skrAccessSecretRepo := &skrAccessSecretRepoStub{
		exists: true,
	}

	uc := usecases.NewDeleteSkrKyma(skrKymaRepo, skrAccessSecretRepo)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.NoError(t, err)
	assert.False(t, applicable)
	assert.True(t, skrAccessSecretRepo.called)
	assert.True(t, skrKymaRepo.called)
	assert.Equal(t, kcpKyma.GetNamespacedName(), skrKymaRepo.namespacedName)
}

func TestIsApplicable_KymaNotDeleting(t *testing.T) {
	time := apimetav1.NewTime(time.Now())
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:              random.Name(),
			Namespace:         random.Name(),
			DeletionTimestamp: &time,
		},
	}

	skrKymaRepo := &skrKymaRepoStub{
		isDeleting: false,
	}

	skrAccessSecretRepo := &skrAccessSecretRepoStub{
		exists: true,
	}

	uc := usecases.NewDeleteSkrKyma(skrKymaRepo, skrAccessSecretRepo)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.NoError(t, err)
	assert.True(t, applicable)
	assert.True(t, skrAccessSecretRepo.called)
	assert.True(t, skrKymaRepo.called)
	assert.Equal(t, kcpKyma.GetNamespacedName(), skrKymaRepo.namespacedName)
}

func TestIsApplicable_IsDeletingFails(t *testing.T) {
	time := apimetav1.NewTime(time.Now())
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:              random.Name(),
			Namespace:         random.Name(),
			DeletionTimestamp: &time,
		},
	}

	skrKymaRepo := &skrKymaRepoStub{
		err: assert.AnError,
	}

	skrAccessSecretRepo := &skrAccessSecretRepoStub{
		exists: true,
	}

	uc := usecases.NewDeleteSkrKyma(skrKymaRepo, skrAccessSecretRepo)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.ErrorIs(t, err, assert.AnError)
	assert.False(t, applicable)
	assert.True(t, skrAccessSecretRepo.called)
	assert.True(t, skrKymaRepo.called)
	assert.Equal(t, kcpKyma.GetNamespacedName(), skrKymaRepo.namespacedName)
}

func TestExecute_DeleteSucceeds(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	skrKymaRepo := &skrKymaRepoStub{}

	uc := usecases.NewDeleteSkrKyma(skrKymaRepo, nil)

	res := uc.Execute(t.Context(), kcpKyma)

	require.NoError(t, res.Err)
	assert.Equal(t, usecase.DeleteSkrKyma, res.UseCase)
	assert.True(t, skrKymaRepo.called)
	assert.Equal(t, kcpKyma.GetNamespacedName(), skrKymaRepo.namespacedName)
}

func TestExecute_DeleteFails(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	skrKymaRepo := &skrKymaRepoStub{
		err: assert.AnError,
	}

	uc := usecases.NewDeleteSkrKyma(skrKymaRepo, nil)

	res := uc.Execute(t.Context(), kcpKyma)

	require.ErrorIs(t, res.Err, assert.AnError)
	assert.Equal(t, usecase.DeleteSkrKyma, res.UseCase)
	assert.True(t, skrKymaRepo.called)
	assert.Equal(t, kcpKyma.GetNamespacedName(), skrKymaRepo.namespacedName)
}

type skrKymaRepoStub struct {
	usecases.SkrKymaRepo

	called         bool
	namespacedName types.NamespacedName
	isDeleting     bool
	err            error
}

func (r *skrKymaRepoStub) IsDeleting(_ context.Context, namespacedName types.NamespacedName) (bool, error) {
	r.called = true
	r.namespacedName = namespacedName
	return r.isDeleting, r.err
}

func (r *skrKymaRepoStub) Delete(_ context.Context, namespacedName types.NamespacedName) error {
	r.called = true
	r.namespacedName = namespacedName
	return r.err
}
