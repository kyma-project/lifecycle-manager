package usecases

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/service/accessmanager"
)

//nolint:iface // we accept the duplication for clarity
type CrdRepo interface {
	Exists(ctx context.Context, kymaName types.NamespacedName) (bool, error)
	Delete(ctx context.Context, kymaName types.NamespacedName) error
}

type DeleteSkrCrd struct {
	skrCrdRepo CrdRepo
	useCase    result.UseCase
}

func NewDeleteSkrCrd(skrCrdRepo CrdRepo, useCase result.UseCase) *DeleteSkrCrd {
	return &DeleteSkrCrd{
		skrCrdRepo: skrCrdRepo,
		useCase:    useCase,
	}
}

func (u *DeleteSkrCrd) IsApplicable(ctx context.Context, kcpKyma *v1beta2.Kyma) (bool, error) {
	if kcpKyma.DeletionTimestamp.IsZero() {
		return false, nil
	}

	exists, err := u.skrCrdRepo.Exists(ctx, kcpKyma.GetNamespacedName())
	if errors.Is(err, accessmanager.ErrAccessSecretNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return exists, nil
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
