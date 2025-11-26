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

func TestDeleteSkrModuleMetata_IsApplicable_False_KcpKymaNotDeleting(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	uc := usecases.NewDeleteSKRModuleMetadata(nil, nil, nil)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.NoError(t, err)
	assert.False(t, applicable)
}

func TestDeleteSkrModuleMetata_IsApplicable_False_SecretDoesNotExist(t *testing.T) {
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

	uc := usecases.NewDeleteSKRModuleMetadata(nil, nil, skrAccessSecretRepo)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.NoError(t, err)
	assert.False(t, applicable)
	assert.True(t, skrAccessSecretRepo.called)
	assert.Equal(t, kcpKyma.GetName(), skrAccessSecretRepo.kymaName)
}

func TestDeleteSkrModuleMetata_IsApplicable_True_MtCrdExists(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:              random.Name(),
			Namespace:         random.Name(),
			DeletionTimestamp: &apimetav1.Time{Time: time.Now()},
		},
	}

	mtRepo := &crdRepoStub{
		exists: true,
	}

	mrmRepo := &crdRepoStub{
		exists: false,
	}

	skrAccessSecretRepo := &skrAccessSecretRepoStub{
		exists: true,
	}

	uc := usecases.NewDeleteSKRModuleMetadata(mtRepo, mrmRepo, skrAccessSecretRepo)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.NoError(t, err)
	assert.True(t, applicable)
	assert.True(t, skrAccessSecretRepo.called)
	assert.True(t, mtRepo.existsCalled)
	assert.False(t, mrmRepo.existsCalled)
	assert.Equal(t, kcpKyma.GetNamespacedName(), mtRepo.namespacedName)
}

func TestDeleteSkrModuleMetata_IsApplicable_True_MrmCrdExists(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:              random.Name(),
			Namespace:         random.Name(),
			DeletionTimestamp: &apimetav1.Time{Time: time.Now()},
		},
	}

	mtRepo := &crdRepoStub{
		exists: false,
	}

	mrmRepo := &crdRepoStub{
		exists: true,
	}

	skrAccessSecretRepo := &skrAccessSecretRepoStub{
		exists: true,
	}

	uc := usecases.NewDeleteSKRModuleMetadata(mtRepo, mrmRepo, skrAccessSecretRepo)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.NoError(t, err)
	assert.True(t, applicable)
	assert.True(t, skrAccessSecretRepo.called)
	assert.True(t, mtRepo.existsCalled)
	assert.True(t, mrmRepo.existsCalled)
	assert.Equal(t, kcpKyma.GetNamespacedName(), mtRepo.namespacedName)
}

func TestDeleteSkrModuleMetata_IsApplicable_False_NoCrdExists(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:              random.Name(),
			Namespace:         random.Name(),
			DeletionTimestamp: &apimetav1.Time{Time: time.Now()},
		},
	}

	mtRepo := &crdRepoStub{
		exists: false,
	}

	mrmRepo := &crdRepoStub{
		exists: false,
	}

	skrAccessSecretRepo := &skrAccessSecretRepoStub{
		exists: true,
	}

	uc := usecases.NewDeleteSKRModuleMetadata(mtRepo, mrmRepo, skrAccessSecretRepo)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.NoError(t, err)
	assert.False(t, applicable)
	assert.True(t, skrAccessSecretRepo.called)
	assert.True(t, mtRepo.existsCalled)
	assert.True(t, mrmRepo.existsCalled)
	assert.Equal(t, kcpKyma.GetNamespacedName(), mtRepo.namespacedName)
	assert.Equal(t, kcpKyma.GetNamespacedName(), mrmRepo.namespacedName)
}

func TestDeleteSkrModuleMetata_Execute_DeleteSucceeds(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	mtRepo := &crdRepoStub{}
	mrmRepo := &crdRepoStub{}

	uc := usecases.NewDeleteSKRModuleMetadata(mtRepo, mrmRepo, nil)

	res := uc.Execute(t.Context(), kcpKyma)

	require.NoError(t, res.Err)
	assert.Equal(t, usecase.DeleteSkrModuleMetadata, res.UseCase)
	assert.True(t, mtRepo.deleteCalled)
	assert.True(t, mrmRepo.deleteCalled)
	assert.Equal(t, kcpKyma.GetNamespacedName(), mtRepo.namespacedName)
	assert.Equal(t, kcpKyma.GetNamespacedName(), mrmRepo.namespacedName)
}

func TestDeleteSkrModuleMetata_Execute_DeleteMtFails(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	mtRepo := &crdRepoStub{
		err: assert.AnError,
	}

	mrmRepo := &crdRepoStub{}

	uc := usecases.NewDeleteSKRModuleMetadata(mtRepo, mrmRepo, nil)

	res := uc.Execute(t.Context(), kcpKyma)

	require.ErrorIs(t, res.Err, assert.AnError)
	assert.Equal(t, usecase.DeleteSkrModuleMetadata, res.UseCase)
	assert.True(t, mtRepo.deleteCalled)
	assert.False(t, mrmRepo.deleteCalled)
	assert.Equal(t, kcpKyma.GetNamespacedName(), mtRepo.namespacedName)
}

func TestDeleteSkrModuleMetata_Execute_DeleteMrmFails(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	mtRepo := &crdRepoStub{}

	mrmRepo := &crdRepoStub{
		err: assert.AnError,
	}

	uc := usecases.NewDeleteSKRModuleMetadata(mtRepo, mrmRepo, nil)

	res := uc.Execute(t.Context(), kcpKyma)

	require.ErrorIs(t, res.Err, assert.AnError)
	assert.Equal(t, usecase.DeleteSkrModuleMetadata, res.UseCase)
	assert.True(t, mtRepo.deleteCalled)
	assert.True(t, mrmRepo.deleteCalled)
	assert.Equal(t, kcpKyma.GetNamespacedName(), mtRepo.namespacedName)
}

type crdRepoStub struct {
	usecases.CrdRepo

	existsCalled   bool
	deleteCalled   bool
	namespacedName types.NamespacedName
	exists         bool
	err            error
}

func (r *crdRepoStub) Exists(_ context.Context, namespacedName types.NamespacedName) (bool, error) {
	r.existsCalled = true
	r.namespacedName = namespacedName
	return r.exists, r.err
}

func (r *crdRepoStub) Delete(_ context.Context, namespacedName types.NamespacedName) error {
	r.deleteCalled = true
	r.namespacedName = namespacedName
	return r.err
}
