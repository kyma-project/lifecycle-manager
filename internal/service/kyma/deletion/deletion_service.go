package deletion

import (
	"context"
	"errors"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/errors/kyma/deletion"
	"github.com/kyma-project/lifecycle-manager/internal/result"
)

var ErrUnableToDetermineUsecaseApplicability = errors.New("unable to determine usecase applicability")

type UseCase interface {
	IsApplicable(ctx context.Context, mrm *v1beta2.Kyma) (bool, error)
	Execute(ctx context.Context, mrm *v1beta2.Kyma) result.Result
	Name() result.UseCase
}

type Service struct {
	deletionSteps []UseCase
}

func NewService(
	setKcpKymaStateDeleting UseCase,
	setSkrKymaStateDeleting UseCase,
	deleteSkrKyma UseCase,
	removeCertificateSetup UseCase,
	removeSkrWebhook UseCase,
	deleteSkrMtCrd UseCase,
	deleteSkrMrmCrd UseCase,
	deleteSkrKymaCrd UseCase,
	deleteManifests UseCase,
	deleteMetrics UseCase,
	dropKymaFinalizers UseCase,
) *Service {
	return &Service{
		deletionSteps: []UseCase{
			setKcpKymaStateDeleting,
			setSkrKymaStateDeleting,
			deleteSkrKyma,
			removeCertificateSetup,
			removeSkrWebhook,
			deleteSkrMtCrd,
			deleteSkrMrmCrd,
			deleteSkrKymaCrd,
			deleteManifests,
			deleteMetrics,
			dropKymaFinalizers,
		},
	}
}

func (s *Service) Delete(ctx context.Context, kyma *v1beta2.Kyma) result.Result {
	for _, step := range s.deletionSteps {
		isApplicable, err := step.IsApplicable(ctx, kyma)
		if err != nil {
			return result.Result{
				UseCase: step.Name(),
				Err:     errors.Join(ErrUnableToDetermineUsecaseApplicability, err),
			}
		}
		if isApplicable {
			return step.Execute(ctx, kyma)
		}
	}

	return result.Result{
		Err: deletion.ErrNoUseCaseApplicable,
	}
}
