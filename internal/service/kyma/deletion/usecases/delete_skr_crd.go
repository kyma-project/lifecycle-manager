package usecases

import (
	"context"

	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
)

type CrdRepo interface {
	Exists(ctx context.Context, kymaName types.NamespacedName) (bool, error)
	Delete(ctx context.Context, kymaName types.NamespacedName) error
}

type DeleteSkrModuleMetadata struct {
	skrCrdRepo          CrdRepo
	skrAccessSecretRepo SkrAccessSecretRepo
	useCase             result.UseCase
}

func NewDeleteSkrCrd(skrCrdRepo CrdRepo,
	skrAccessSecretRepo SkrAccessSecretRepo,
	useCase result.UseCase,
) *DeleteSkrModuleMetadata {
	return &DeleteSkrModuleMetadata{
		skrCrdRepo:          skrCrdRepo,
		skrAccessSecretRepo: skrAccessSecretRepo,
		useCase:             useCase,
	}
}

func (u *DeleteSkrModuleMetadata) IsApplicable(ctx context.Context, kcpKyma *v1beta2.Kyma) (bool, error) {
	if kcpKyma.DeletionTimestamp.IsZero() {
		return false, nil
	}

	if exists, err := u.skrAccessSecretRepo.Exists(ctx, kcpKyma.GetName()); !exists || err != nil {
		return false, err
	}

	return u.skrCrdRepo.Exists(ctx,
		kcpKyma.GetNamespacedName(),
	)
}

func (u *DeleteSkrModuleMetadata) Execute(ctx context.Context, kcpKyma *v1beta2.Kyma) result.Result {
	// deleting the CRDs is sufficient as this also deletes related CRs
	return result.Result{
		UseCase: u.Name(),
		Err:     u.skrCrdRepo.Delete(ctx, kcpKyma.GetNamespacedName()),
	}
}

func (u *DeleteSkrModuleMetadata) Name() result.UseCase {
	return u.useCase
}
