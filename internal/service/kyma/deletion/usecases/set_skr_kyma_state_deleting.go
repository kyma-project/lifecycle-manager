package usecases

import (
	"context"

	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
)

type SkrKymaStatusRepo interface {
	SetStateDeleting(ctx context.Context, namespacedName types.NamespacedName) error
	Get(ctx context.Context, namespacedName types.NamespacedName) (*v1beta2.KymaStatus, error)
}

type SetSkrKymaStateDeleting struct {
	skrKymaStatusRepo   SkrKymaStatusRepo
	skrAccessSecretRepo SkrAccessSecretRepo
}

func NewSetSkrKymaStateDeleting(kymaStatusRepo SkrKymaStatusRepo,
	skrAccessSecretRepo SkrAccessSecretRepo,
) *SetSkrKymaStateDeleting {
	return &SetSkrKymaStateDeleting{
		skrKymaStatusRepo:   kymaStatusRepo,
		skrAccessSecretRepo: skrAccessSecretRepo,
	}
}

func (u *SetSkrKymaStateDeleting) IsApplicable(ctx context.Context, kcpKyma *v1beta2.Kyma) (bool, error) {
	if kcpKyma.DeletionTimestamp.IsZero() {
		return false, nil
	}

	if exists, err := u.skrAccessSecretRepo.Exists(ctx, kcpKyma.GetName()); !exists || err != nil {
		return false, err
	}

	status, err := u.skrKymaStatusRepo.Get(ctx, kcpKyma.GetNamespacedName())
	if err != nil {
		return false, err
	}

	return status.State != shared.StateDeleting, nil
}

func (u *SetSkrKymaStateDeleting) Execute(ctx context.Context, kcpKyma *v1beta2.Kyma) result.Result {
	return result.Result{
		UseCase: u.Name(),
		Err:     u.skrKymaStatusRepo.SetStateDeleting(ctx, kcpKyma.GetNamespacedName()),
	}
}

func (u *SetSkrKymaStateDeleting) Name() result.UseCase {
	return usecase.SetSkrKymaStateDeleting
}
