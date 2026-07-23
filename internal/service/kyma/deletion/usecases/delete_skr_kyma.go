package usecases

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
	"github.com/kyma-project/lifecycle-manager/internal/service/accessmanager"
)

//nolint:iface // we accept the duplication for clarity
type SkrKymaRepo interface {
	Exists(ctx context.Context, kymaName types.NamespacedName) (bool, error)
	Delete(ctx context.Context, kymaName types.NamespacedName) error
}

type DeleteSkrKyma struct {
	skrKymaRepo SkrKymaRepo
}

func NewDeleteSkrKyma(skrKymaRepo SkrKymaRepo) *DeleteSkrKyma {
	return &DeleteSkrKyma{
		skrKymaRepo: skrKymaRepo,
	}
}

func (u *DeleteSkrKyma) IsApplicable(ctx context.Context, kcpKyma *v1beta2.Kyma) (bool, error) {
	if kcpKyma.DeletionTimestamp.IsZero() {
		return false, nil
	}

	exists, err := u.skrKymaRepo.Exists(ctx, kcpKyma.GetNamespacedName())
	if errors.Is(err, accessmanager.ErrAccessSecretNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return exists, nil
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
