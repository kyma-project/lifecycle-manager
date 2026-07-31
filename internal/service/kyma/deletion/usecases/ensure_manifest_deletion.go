package usecases

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
)

type EnsureManifestDeletionRepo interface {
	ExistForKyma(ctx context.Context, kymaName string) (bool, error)
}

type EnsureManifestDeletion struct {
	ensureManifestDeletionRepo EnsureManifestDeletionRepo
}

func NewEnsureManifestDeletion(ensureManifestDeletionRepo EnsureManifestDeletionRepo) *EnsureManifestDeletion {
	return &EnsureManifestDeletion{
		ensureManifestDeletionRepo: ensureManifestDeletionRepo,
	}
}

func (u *EnsureManifestDeletion) IsApplicable(ctx context.Context, kcpKyma *v1beta2.Kyma) (bool, error) {
	return u.ensureManifestDeletionRepo.ExistForKyma(ctx, kcpKyma.GetName())
}

func (u *EnsureManifestDeletion) Execute(_ context.Context, _ *v1beta2.Kyma) result.Result {
	// no-op
	return result.Result{
		UseCase: usecase.EnsureManifestDeletion,
		Err:     nil,
	}
}

func (u *EnsureManifestDeletion) Name() result.UseCase {
	return usecase.EnsureManifestDeletion
}
