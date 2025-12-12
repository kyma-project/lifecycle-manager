package usecases

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
)

const kymaFinalizer = shared.KymaFinalizer

type KymaRepo interface {
	DropFinalizer(ctx context.Context, kymaName string, finalizer string) error
}

type DropKymaFinalizer struct {
	kymaRepo KymaRepo
}

func NewDropKymaFinalizer(kymaRepo KymaRepo) *DropKymaFinalizer {
	return &DropKymaFinalizer{
		kymaRepo: kymaRepo,
	}
}

func (u *DropKymaFinalizer) IsApplicable(ctx context.Context, kcpKyma *v1beta2.Kyma) (bool, error) {
	return controllerutil.ContainsFinalizer(kcpKyma, shared.KymaFinalizer), nil
}

func (u *DropKymaFinalizer) Execute(ctx context.Context, kcpKyma *v1beta2.Kyma) result.Result {
	return result.Result{
		UseCase: u.Name(),
		Err:     u.kymaRepo.DropFinalizer(ctx, kcpKyma.GetName(), kymaFinalizer),
	}
}

func (u *DropKymaFinalizer) Name() result.UseCase {
	return usecase.DropKymaFinalizer
}
