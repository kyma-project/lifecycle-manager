package usecases

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
)

type CrdRepo = ExistsDeleteRepo

type DeleteSkrCrd struct {
	skrCrdRepo          CrdRepo
	skrAccessSecretRepo SkrAccessSecretRepo
	useCase             result.UseCase
}

func NewDeleteSkrCrd(skrCrdRepo CrdRepo,
	skrAccessSecretRepo SkrAccessSecretRepo,
	useCase result.UseCase,
) *DeleteSkrCrd {
	return &DeleteSkrCrd{
		skrCrdRepo:          skrCrdRepo,
		skrAccessSecretRepo: skrAccessSecretRepo,
		useCase:             useCase,
	}
}

func (u *DeleteSkrCrd) IsApplicable(ctx context.Context, kcpKyma *v1beta2.Kyma) (bool, error) {
	if kcpKyma.DeletionTimestamp.IsZero() {
		return false, nil
	}

	if exists, err := u.skrAccessSecretRepo.ExistsForKyma(ctx, kcpKyma.GetName()); !exists || err != nil {
		return false, err
	}

	return u.skrCrdRepo.Exists(ctx,
		kcpKyma.GetNamespacedName(),
	)
}

func (u *DeleteSkrCrd) Execute(ctx context.Context, kcpKyma *v1beta2.Kyma) result.Result {
	// deleting the CRD also deletes related CRs
	return result.Result{
		UseCase: u.Name(),
		Err:     u.skrCrdRepo.Delete(ctx, kcpKyma.GetNamespacedName()),
	}
}

func (u *DeleteSkrCrd) Name() result.UseCase {
	return u.useCase
}
