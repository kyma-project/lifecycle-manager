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

func Test_DeleteManifests_IsApplicable_ManifestsExist(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	manifestRepo := &manifestRepoStub{
		exists: true,
	}

	uc := usecases.NewDeleteManifests(manifestRepo)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.NoError(t, err)
	assert.True(t, applicable)
	assert.True(t, manifestRepo.called)
	assert.Equal(t, kcpKyma.GetName(), manifestRepo.kymaName)
}

func Test_DeleteManifests_IsApplicable_ManifestsDoNotExist(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	manifestRepo := &manifestRepoStub{
		exists: false,
	}

	uc := usecases.NewDeleteManifests(manifestRepo)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.NoError(t, err)
	assert.False(t, applicable)
	assert.True(t, manifestRepo.called)
	assert.Equal(t, kcpKyma.GetName(), manifestRepo.kymaName)
}

func Test_DeleteManifests_IsApplicable_ExistFails(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	manifestRepo := &manifestRepoStub{
		err: assert.AnError,
	}

	uc := usecases.NewDeleteManifests(manifestRepo)

	applicable, err := uc.IsApplicable(t.Context(), kcpKyma)

	require.ErrorIs(t, err, assert.AnError)
	assert.False(t, applicable)
	assert.True(t, manifestRepo.called)
	assert.Equal(t, kcpKyma.GetName(), manifestRepo.kymaName)
}

func Test_DeleteManifests_Execute_DeleteSucceeds(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	manifestRepo := &manifestRepoStub{}

	uc := usecases.NewDeleteManifests(manifestRepo)

	res := uc.Execute(t.Context(), kcpKyma)

	require.NoError(t, res.Err)
	assert.Equal(t, usecase.DeleteManifests, res.UseCase)
	assert.True(t, manifestRepo.deleteCalled)
	assert.Equal(t, kcpKyma.GetName(), manifestRepo.kymaName)
}

func Test_DeleteManifests_Execute_DeleteFails(t *testing.T) {
	kcpKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	manifestRepo := &manifestRepoStub{
		deleteErr: assert.AnError,
	}

	uc := usecases.NewDeleteManifests(manifestRepo)

	res := uc.Execute(t.Context(), kcpKyma)

	require.ErrorIs(t, res.Err, assert.AnError)
	assert.Equal(t, usecase.DeleteManifests, res.UseCase)
	assert.True(t, manifestRepo.deleteCalled)
	assert.Equal(t, kcpKyma.GetName(), manifestRepo.kymaName)
}

type manifestRepoStub struct {
	usecases.ManifestRepo

	called       bool
	deleteCalled bool
	kymaName     string
	exists       bool
	err          error
	deleteErr    error
}

func (r *manifestRepoStub) ExistForKyma(_ context.Context, kymaName string) (bool, error) {
	r.called = true
	r.kymaName = kymaName
	return r.exists, r.err
}

func (r *manifestRepoStub) DeleteAllForKyma(_ context.Context, kymaName string) error {
	r.deleteCalled = true
	r.kymaName = kymaName
	return r.deleteErr
}
