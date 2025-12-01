package usecases

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
)

type ManifestRepo interface {
	ExistForKyma(ctx context.Context, kymaName string) (bool, error)
	DeleteAllForKyma(ctx context.Context, kymaName string) error
}

type DeleteManifests struct {
	manifestRepo ManifestRepo
}

func NewDeleteManifests(manifestRepo ManifestRepo) *DeleteManifests {
	return &DeleteManifests{
		manifestRepo: manifestRepo,
	}
}

func (u *DeleteManifests) IsApplicable(ctx context.Context, kcpKyma *v1beta2.Kyma) (bool, error) {
	return u.manifestRepo.ExistForKyma(ctx, kcpKyma.GetName())
}

func (u *DeleteManifests) Execute(ctx context.Context, kcpKyma *v1beta2.Kyma) result.Result {
	return result.Result{
		UseCase: u.Name(),
		Err:     u.manifestRepo.DeleteAllForKyma(ctx, kcpKyma.GetName()),
	}
}

func (u *DeleteManifests) Name() result.UseCase {
	return usecase.DeleteManifests
}
