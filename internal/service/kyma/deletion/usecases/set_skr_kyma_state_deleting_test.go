package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/deletion/usecases"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestExecute_Succeeds(t *testing.T) {
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	kymaStatusRepo := &skrKymaStatusRepoStub{}

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

	kymaStatusRepo := &skrKymaStatusRepoStub{
		err: assert.AnError,
	}

	uc := usecases.NewSetSkrKymaStateDeleting(kymaStatusRepo, nil)

	result := uc.Execute(t.Context(), kyma)

	require.ErrorIs(t, result.Err, assert.AnError)
	assert.Equal(t, usecase.SetSkrKymaStateDeleting, result.UseCase)
	assert.True(t, kymaStatusRepo.called)
	assert.Equal(t, kyma.GetNamespacedName(), kymaStatusRepo.namespacedName)
}

func TestIsApplicable_False_DeletionTimestampNotSet(t *testing.T) {
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      random.Name(),
			Namespace: random.Name(),
		},
	}

	skrAccessSecretRepoStub := &skrAccessSecretRepoStub{}

	uc := usecases.NewSetSkrKymaStateDeleting(nil, skrAccessSecretRepoStub)

	applicable, err := uc.IsApplicable(t.Context(), kyma)

	require.NoError(t, err)
	assert.False(t, applicable)
	assert.False(t, skrAccessSecretRepoStub.called)
}

func TestIsApplicable_False_SecretDoesNotExist(t *testing.T) {
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:              random.Name(),
			Namespace:         random.Name(),
			DeletionTimestamp: &apimetav1.Time{Time: time.Now()},
		},
	}

	skrAccessSecretRepo := &skrAccessSecretRepoStub{
		exists: false,
	}

	uc := usecases.NewSetSkrKymaStateDeleting(nil, skrAccessSecretRepo)

	applicable, err := uc.IsApplicable(t.Context(), kyma)

	require.NoError(t, err)
	assert.False(t, applicable)
	assert.True(t, skrAccessSecretRepo.called)
	assert.Equal(t, kyma.GetName(), skrAccessSecretRepo.kymaName)
}

func TestIsApplicable_False_SecretRepoReturnsError(t *testing.T) {
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:              random.Name(),
			Namespace:         random.Name(),
			DeletionTimestamp: &apimetav1.Time{Time: time.Now()},
		},
	}

	skrAccessSecretRepo := &skrAccessSecretRepoStub{
		err: assert.AnError,
	}

	uc := usecases.NewSetSkrKymaStateDeleting(nil, skrAccessSecretRepo)

	applicable, err := uc.IsApplicable(t.Context(), kyma)

	require.ErrorIs(t, err, assert.AnError)
	assert.False(t, applicable)
	assert.True(t, skrAccessSecretRepo.called)
	assert.Equal(t, kyma.GetName(), skrAccessSecretRepo.kymaName)
}

func TestIsApplicable_False_KymaAlreadyInDeletingState(t *testing.T) {
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:              random.Name(),
			Namespace:         random.Name(),
			DeletionTimestamp: &apimetav1.Time{Time: time.Now()},
		},
	}

	kymaStatusRepo := &skrKymaStatusRepoStub{
		status: &v1beta2.KymaStatus{
			State: shared.StateDeleting,
		},
	}

	skrAccessSecretRepo := &skrAccessSecretRepoStub{
		exists: true,
	}

	uc := usecases.NewSetSkrKymaStateDeleting(kymaStatusRepo, skrAccessSecretRepo)

	applicable, err := uc.IsApplicable(t.Context(), kyma)

	require.NoError(t, err)
	assert.False(t, applicable)
	assert.True(t, skrAccessSecretRepo.called)
	assert.True(t, kymaStatusRepo.called)
	assert.Equal(t, kyma.GetNamespacedName(), kymaStatusRepo.namespacedName)
}

func TestIsApplicable_False_KymaAlreadyGone(t *testing.T) {
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:              random.Name(),
			Namespace:         random.Name(),
			DeletionTimestamp: &apimetav1.Time{Time: time.Now()},
		},
	}

	kymaStatusRepo := &skrKymaStatusRepoStub{
		err: apierrors.NewNotFound(
			apicorev1.Resource(string(shared.KymaKind)),
			shared.DefaultRemoteKymaName,
		),
	}

	skrAccessSecretRepo := &skrAccessSecretRepoStub{
		exists: true,
	}

	uc := usecases.NewSetSkrKymaStateDeleting(kymaStatusRepo, skrAccessSecretRepo)

	applicable, err := uc.IsApplicable(t.Context(), kyma)

	require.NoError(t, err)
	assert.False(t, applicable)
	assert.True(t, skrAccessSecretRepo.called)
	assert.True(t, kymaStatusRepo.called)
	assert.Equal(t, kyma.GetNamespacedName(), kymaStatusRepo.namespacedName)
}

func TestIsApplicable_True(t *testing.T) {
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:              random.Name(),
			Namespace:         random.Name(),
			DeletionTimestamp: &apimetav1.Time{Time: time.Now()},
		},
	}

	kymaStatusRepo := &skrKymaStatusRepoStub{
		status: &v1beta2.KymaStatus{
			State: shared.StateReady,
		},
	}

	skrAccessSecretRepo := &skrAccessSecretRepoStub{
		exists: true,
	}

	uc := usecases.NewSetSkrKymaStateDeleting(kymaStatusRepo, skrAccessSecretRepo)

	applicable, err := uc.IsApplicable(t.Context(), kyma)

	require.NoError(t, err)
	assert.True(t, applicable)
	assert.True(t, skrAccessSecretRepo.called)
	assert.True(t, kymaStatusRepo.called)
	assert.Equal(t, kyma.GetNamespacedName(), kymaStatusRepo.namespacedName)
}

type skrAccessSecretRepoStub struct {
	usecases.SkrAccessSecretRepo

	called   bool
	kymaName string
	exists   bool
	err      error
}

func (r *skrAccessSecretRepoStub) Exists(_ context.Context, kymaName string) (bool, error) {
	r.called = true
	r.kymaName = kymaName
	return r.exists, r.err
}

type skrKymaStatusRepoStub struct {
	usecases.SkrKymaStatusRepo

	called         bool
	namespacedName types.NamespacedName
	status         *v1beta2.KymaStatus
	err            error
}

func (r *skrKymaStatusRepoStub) Get(_ context.Context,
	namespacedName types.NamespacedName,
) (*v1beta2.KymaStatus, error) {
	r.called = true
	r.namespacedName = namespacedName
	return r.status, r.err
}

func (r *skrKymaStatusRepoStub) SetStateDeleting(_ context.Context, namespacedName types.NamespacedName) error {
	r.called = true
	r.namespacedName = namespacedName
	return r.err
}
