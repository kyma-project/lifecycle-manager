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
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/deletion/usecases"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestDeleteSkrCrd_Name_ReturnsExpectedName(t *testing.T) {
	ucName := result.UseCase(random.Name())
	uc := usecases.NewDeleteSkrCrd(nil, nil, ucName)

	assert.Equal(t, ucName, uc.Name())
}

func TestDeleteSkrCrd_IsApplicable_False_KcpKymaNotDeleting(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	uc := usecases.NewDeleteSkrCrd(nil, nil, result.UseCase(random.Name()))

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.NoError(t, err)
	assert.False(t, applicable)
}

func TestDeleteSkrCrd_IsApplicable_False_SecretDoesNotExist(t *testing.T) {
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

	uc := usecases.NewDeleteSkrCrd(nil, skrAccessSecretRepo, result.UseCase(random.Name()))

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.NoError(t, err)
	assert.False(t, applicable)
	assert.True(t, skrAccessSecretRepo.called)
	assert.Equal(t, kcpKyma.GetName(), skrAccessSecretRepo.kymaName)
}

func TestDeleteSkrCrd_IsApplicable_False_SecretRepoReturnsError(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:              random.Name(),
			Namespace:         random.Name(),
			DeletionTimestamp: &apimetav1.Time{Time: time.Now()},
		},
	}

	skrAccessSecretRepo := &skrAccessSecretRepoStub{
		err: assert.AnError,
	}

	uc := usecases.NewDeleteSkrCrd(nil, skrAccessSecretRepo, result.UseCase(random.Name()))

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.ErrorIs(t, err, assert.AnError)
	assert.False(t, applicable)
	assert.True(t, skrAccessSecretRepo.called)
	assert.Equal(t, kcpKyma.GetName(), skrAccessSecretRepo.kymaName)
}

func TestDeleteSkrCrd_IsApplicable_True_CrdExists(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:              random.Name(),
			Namespace:         random.Name(),
			DeletionTimestamp: &apimetav1.Time{Time: time.Now()},
		},
	}

	crdRepo := &crdRepoStub{
		exists: true,
	}

	skrAccessSecretRepo := &skrAccessSecretRepoStub{
		exists: true,
	}

	uc := usecases.NewDeleteSkrCrd(crdRepo, skrAccessSecretRepo, result.UseCase(random.Name()))

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.NoError(t, err)
	assert.True(t, applicable)
	assert.True(t, skrAccessSecretRepo.called)
	assert.True(t, crdRepo.existsCalled)
	assert.Equal(t, kcpKyma.GetNamespacedName(), crdRepo.namespacedName)
}

func TestDeleteSkrModuleMetata_Execute_DeleteSucceeds(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}
	crdRepo := &crdRepoStub{}
	ucName := result.UseCase(random.Name())

	uc := usecases.NewDeleteSkrCrd(crdRepo, nil, ucName)

	res := uc.Execute(t.Context(), kcpKyma)

	require.NoError(t, res.Err)
	assert.Equal(t, ucName, res.UseCase)
	assert.True(t, crdRepo.deleteCalled)
	assert.Equal(t, kcpKyma.GetNamespacedName(), crdRepo.namespacedName)
}

func TestDeleteSkrModuleMetata_Execute_DeleteFails(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	crdRepo := &crdRepoStub{
		err: assert.AnError,
	}
	ucName := result.UseCase(random.Name())

	uc := usecases.NewDeleteSkrCrd(crdRepo, nil, ucName)

	res := uc.Execute(t.Context(), kcpKyma)

	require.ErrorIs(t, res.Err, assert.AnError)
	assert.Equal(t, ucName, res.UseCase)
	assert.True(t, crdRepo.deleteCalled)
	assert.Equal(t, kcpKyma.GetNamespacedName(), crdRepo.namespacedName)
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
