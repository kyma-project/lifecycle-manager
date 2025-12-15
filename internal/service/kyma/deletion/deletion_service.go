package deletion

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/errors/kyma/deletion"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
)

var (
	ErrUnableToDetermineUsecaseApplicability = errors.New("unable to determine usecase applicability")
	ErrUseCasesOutOfOrder                    = errors.New("deletion use cases are not in the expected order")
)

type UseCase interface {
	IsApplicable(ctx context.Context, kyma *v1beta2.Kyma) (bool, error)
	Execute(ctx context.Context, kyma *v1beta2.Kyma) result.Result
	Name() result.UseCase
}

type Service struct {
	deletionSteps []UseCase
}

func NewService(
	setKcpKymaStateDeleting UseCase,
	setSkrKymaStateDeleting UseCase,
	deleteSkrKyma UseCase,
	deleteWatcherCertificateSetup UseCase,
	deleteSkrWebhookResources UseCase,
	deleteSkrMtCrd UseCase,
	deleteSkrMrmCrd UseCase,
	deleteSkrKymaCrd UseCase,
	deleteManifests UseCase,
	deleteMetrics UseCase,
	dropKymaFinalizers UseCase,
) (*Service, error) {
	svc := &Service{
		deletionSteps: []UseCase{
			setKcpKymaStateDeleting,
			setSkrKymaStateDeleting,
			deleteSkrKyma,
			deleteWatcherCertificateSetup,
			deleteSkrWebhookResources,
			deleteSkrMtCrd,
			deleteSkrMrmCrd,
			deleteSkrKymaCrd,
			deleteManifests,
			deleteMetrics,
			dropKymaFinalizers,
		},
	}

	if err := svc.enforceUseCaseOrder(); err != nil {
		return nil, err
	}

	return svc, nil
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

func (s *Service) enforceUseCaseOrder() error {
	expectedUseCaseOrder := []result.UseCase{
		usecase.SetKcpKymaStateDeleting,
		usecase.SetSkrKymaStateDeleting,
		usecase.DeleteSkrKyma,
		usecase.DeleteWatcherCertificateSetup,
		usecase.DeleteSkrWebhookResources,
		usecase.DeleteSkrModuleTemplateCrd,
		usecase.DeleteSkrModuleReleaseMetaCrd,
		usecase.DeleteSkrKymaCrd,
		usecase.DeleteManifests,
		usecase.DeleteMetrics,
		usecase.DropKymaFinalizer,
	}

	var err error
	for idx, expectedUseCase := range expectedUseCaseOrder {
		if s.deletionSteps[idx].Name() != expectedUseCase {
			err = errors.Join(err,
				//nolint:err113 // we are wrapping below
				fmt.Errorf("expected use case %s at position %d but found %s",
					expectedUseCase,
					idx,
					s.deletionSteps[idx].Name(),
				),
			)
		}
	}

	if err != nil {
		return errors.Join(ErrUseCasesOutOfOrder, err)
	}

	return nil
}
