package usecases_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/deletion/usecases"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func Test_EnsureManifestDeletion_IsApplicable_ManifestsExist(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	ensureManifestDeletionRepo := &ensureManifestDeletionRepoStub{
		exists: true,
	}

	uc := usecases.NewEnsureManifestDeletion(ensureManifestDeletionRepo)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.NoError(t, err)
	assert.True(t, applicable)
	assert.True(t, ensureManifestDeletionRepo.called)
	assert.Equal(t, kcpKyma.GetName(), ensureManifestDeletionRepo.kymaName)
}

func Test_EnsureManifestDeletion_IsApplicable_ManifestsDoNotExist(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	ensureManifestDeletionRepo := &ensureManifestDeletionRepoStub{
		exists: false,
	}

	uc := usecases.NewEnsureManifestDeletion(ensureManifestDeletionRepo)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.NoError(t, err)
	assert.False(t, applicable)
	assert.True(t, ensureManifestDeletionRepo.called)
	assert.Equal(t, kcpKyma.GetName(), ensureManifestDeletionRepo.kymaName)
}

func Test_EnsureManifestDeletion_IsApplicable_ExistFails(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	ensureManifestDeletionRepo := &ensureManifestDeletionRepoStub{
		err: assert.AnError,
	}

	uc := usecases.NewEnsureManifestDeletion(ensureManifestDeletionRepo)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.ErrorIs(t, err, assert.AnError)
	assert.False(t, applicable)
	assert.True(t, ensureManifestDeletionRepo.called)
	assert.Equal(t, kcpKyma.GetName(), ensureManifestDeletionRepo.kymaName)
}

func Test_EnsureManifestDeletion_Execute_NoOp(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	uc := usecases.NewEnsureManifestDeletion(nil)

	res := uc.Execute(t.Context(), kcpKyma)

	require.NoError(t, res.Err)
	assert.Equal(t, usecase.EnsureManifestDeletion, res.UseCase)
}

type ensureManifestDeletionRepoStub struct {
	usecases.EnsureManifestDeletionRepo

	called   bool
	kymaName string
	exists   bool
	err      error
}

func (r *ensureManifestDeletionRepoStub) ExistForKyma(_ context.Context, kymaName string) (bool, error) {
	r.called = true
	r.kymaName = kymaName
	return r.exists, r.err
}
