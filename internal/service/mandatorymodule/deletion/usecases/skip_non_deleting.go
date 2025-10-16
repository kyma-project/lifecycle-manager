package usecases

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/errors/mandatorymodule/deletion"
)

// SkipNonDeleting is a use case that skips ModuleReleaseMetas that are not in deleting state.
type SkipNonDeleting struct{}

func NewSkipNonDeleting() *SkipNonDeleting {
	return &SkipNonDeleting{}
}

// IsApplicable returns true if the ModuleReleaseMeta is not in deleting state, it should be skipped.
func (s *SkipNonDeleting) IsApplicable(_ context.Context, mrm *v1beta2.ModuleReleaseMeta) (bool, error) {
	return mrm.DeletionTimestamp.IsZero(), nil
}

func (s *SkipNonDeleting) Execute(_ context.Context, _ *v1beta2.ModuleReleaseMeta) error {
	return deletion.ErrMrmNotInDeletingState
}
