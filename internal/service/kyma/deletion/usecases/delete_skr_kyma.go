package usecases

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
)

type SkrKymaRepo = ExistsDeleteByKymaNameRepo

type DeleteSkrKyma struct {
	skrKymaRepo         SkrKymaRepo
	skrAccessSecretRepo SkrAccessSecretRepo
}

func NewDeleteSkrKyma(skrKymaRepo SkrKymaRepo,
	skrAccessSecretRepo SkrAccessSecretRepo,
) *DeleteSkrKyma {
	return &DeleteSkrKyma{
		skrKymaRepo:         skrKymaRepo,
		skrAccessSecretRepo: skrAccessSecretRepo,
	}
}

func (u *DeleteSkrKyma) IsApplicable(ctx context.Context, kcpKyma *v1beta2.Kyma) (bool, error) {
	if kcpKyma.DeletionTimestamp.IsZero() {
		return false, nil
	}

	if exists, err := u.skrAccessSecretRepo.ExistsForKyma(ctx, kcpKyma.GetName()); !exists || err != nil {
		return false, err
	}

	return u.skrKymaRepo.Exists(ctx, kcpKyma.GetNamespacedName())
}

func (u *DeleteSkrKyma) Execute(ctx context.Context, kcpKyma *v1beta2.Kyma) result.Result {
	return result.Result{
		UseCase: u.Name(),
		Err:     u.skrKymaRepo.Delete(ctx, kcpKyma.GetNamespacedName()),
	}
}

func (u *DeleteSkrKyma) Name() result.UseCase {
	return usecase.DeleteSkrKyma
}
