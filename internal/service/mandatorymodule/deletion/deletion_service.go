package deletion

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type UseCase interface {
	IsApplicable(ctx context.Context, mrm *v1beta2.ModuleReleaseMeta) (bool, error)
	Execute(ctx context.Context, mrm *v1beta2.ModuleReleaseMeta) error
}

type Service struct {
	orderedSteps []UseCase
}

func NewService(skipNonMandatory UseCase,
	ensureFinalizer UseCase,
	skipNonDeleting UseCase,
	deleteManifests UseCase,
	removeFinalizer UseCase,
) *Service {
	return &Service{
		orderedSteps: []UseCase{
			skipNonMandatory, // if returns deletion.ErrMrmNotMandatory, controller should not requeue
			ensureFinalizer,
			skipNonDeleting, // if returns deletion.ErrMrmNotInDeletingState, controller should not requeue
			deleteManifests,
			removeFinalizer,
		},
	}
}

func (s *Service) HandleDeletion(ctx context.Context, mrm *v1beta2.ModuleReleaseMeta) error {
	// Find the first applicable step and execute it
	for _, step := range s.orderedSteps {
		isApplicable, err := step.IsApplicable(ctx, mrm)
		if err != nil {
			return err
		}
		if isApplicable {
			return step.Execute(ctx, mrm)
		}
	}
	return nil
}
