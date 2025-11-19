package usecases

import (
	"context"

	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
)

type KymaStatusRepo interface {
	SetStateDeleting(ctx context.Context, namespacedName types.NamespacedName) error
	Get(ctx context.Context, namespacedName types.NamespacedName) (*v1beta2.KymaStatus, error)
}

type SkrAccessSecretRepo interface {
	Exists(ctx context.Context, kymaName string) (bool, error)
}

type SetSkrKymaStateDeleting struct {
	kymaStatusRepo      KymaStatusRepo
	skrAccessSecretRepo SkrAccessSecretRepo
}

func NewSetSkrKymaStateDeleting(kymaStatusRepo KymaStatusRepo,
	skrAccessSecretRepo SkrAccessSecretRepo,
) *SetSkrKymaStateDeleting {
	return &SetSkrKymaStateDeleting{
		kymaStatusRepo: kymaStatusRepo,
	}
}

func (u *SetSkrKymaStateDeleting) IsApplicable(ctx context.Context, kyma *v1beta2.Kyma) (bool, error) {
	if kyma.DeletionTimestamp.IsZero() {
		return false, nil
	}

	if exists, err := u.skrAccessSecretRepo.Exists(ctx, kyma.GetName()); !exists || err != nil {
		return false, err
	}

	status, err := u.kymaStatusRepo.Get(ctx, kyma.GetNamespacedName())
	if err != nil {
		return false, err
	}

	return status.State != shared.StateDeleting, nil
}

func (u *SetSkrKymaStateDeleting) Execute(ctx context.Context, kyma *v1beta2.Kyma) result.Result {
	return result.Result{
		UseCase: u.Name(),
		Err:     u.kymaStatusRepo.SetStateDeleting(ctx, kyma.GetNamespacedName()),
	}
}

func (u *SetSkrKymaStateDeleting) Name() result.UseCase {
	return usecase.SetSkrKymaStateDeleting
}
