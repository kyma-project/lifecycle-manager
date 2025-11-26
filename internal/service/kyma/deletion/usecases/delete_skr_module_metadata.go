package usecases

import (
	"context"

	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
)

type CrdRepo interface {
	Exists(ctx context.Context, kymaName types.NamespacedName) (bool, error)
	Delete(ctx context.Context, kymaName types.NamespacedName) error
}

type DeleteSkrModuleMetadata struct {
	skrMtRepo           CrdRepo
	skrMrmRepo          CrdRepo
	skrAccessSecretRepo SkrAccessSecretRepo
}

func NewDeleteSKRModuleMetadata(skrMtRepo CrdRepo,
	skrMrmRepo CrdRepo,
	skrAccessSecretRepo SkrAccessSecretRepo,
) *DeleteSkrModuleMetadata {
	return &DeleteSkrModuleMetadata{
		skrMtRepo:           skrMtRepo,
		skrMrmRepo:          skrMrmRepo,
		skrAccessSecretRepo: skrAccessSecretRepo,
	}
}

func (u *DeleteSkrModuleMetadata) IsApplicable(ctx context.Context, kcpKyma *v1beta2.Kyma) (bool, error) {
	if kcpKyma.DeletionTimestamp.IsZero() {
		return false, nil
	}

	if exists, err := u.skrAccessSecretRepo.Exists(ctx, kcpKyma.GetName()); !exists || err != nil {
		return false, err
	}

	if mtCrdExists, err := u.skrMtRepo.Exists(ctx,
		kcpKyma.GetNamespacedName(),
	); mtCrdExists || err != nil {
		return mtCrdExists, err
	}

	if mrmCrdExists, err := u.skrMrmRepo.Exists(ctx,
		kcpKyma.GetNamespacedName(),
	); mrmCrdExists || err != nil {
		return mrmCrdExists, err
	}

	return false, nil
}

func (u *DeleteSkrModuleMetadata) Execute(ctx context.Context, kcpKyma *v1beta2.Kyma) result.Result {
	// deleting the CRDs is sufficient as this also deletes related CRs
	errMt := u.skrMtRepo.Delete(ctx, kcpKyma.GetNamespacedName())
	if errMt != nil {
		return result.Result{
			UseCase: u.Name(),
			Err:     errMt,
		}
	}

	errMrm := u.skrMrmRepo.Delete(ctx, kcpKyma.GetNamespacedName())
	if errMrm != nil {
		return result.Result{
			UseCase: u.Name(),
			Err:     errMrm,
		}
	}

	return result.Result{
		UseCase: u.Name(),
		Err:     nil,
	}
}

func (u *DeleteSkrModuleMetadata) Name() result.UseCase {
	return usecase.DeleteSkrModuleMetadata
}
