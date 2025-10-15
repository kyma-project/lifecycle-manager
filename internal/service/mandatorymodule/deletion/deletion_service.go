package deletion

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type UseCase interface {
	ShouldExecute(ctx context.Context, mrm *v1beta2.ModuleReleaseMeta) (bool, error)
	Execute(ctx context.Context, mrm *v1beta2.ModuleReleaseMeta) error
}

type Service struct {
	orderedUseCases []UseCase
}

func NewService(skipNonMandatory UseCase,
	ensureFinalizer UseCase,
	skipNonDeleting UseCase,
	deleteManifests UseCase,
	removeFinalizer UseCase,
) *Service {
	return &Service{
		orderedUseCases: []UseCase{
			skipNonMandatory, // if returns deletion.ErrMrmNotMandatory, controller should not requeue
			ensureFinalizer,
			skipNonDeleting, // if returns deletion.ErrMrmNotInDeletingState, controller should not requeue
			deleteManifests,
			removeFinalizer,
		},
	}
}

func (s *Service) HandleDeletion(ctx context.Context, mrm *v1beta2.ModuleReleaseMeta) error {
	for _, useCase := range s.orderedUseCases {
		shouldExecute, err := useCase.ShouldExecute(ctx, mrm)
		if err != nil {
			return err
		}
		if shouldExecute {
			return useCase.Execute(ctx, mrm)
		}
	}
	return nil
}
