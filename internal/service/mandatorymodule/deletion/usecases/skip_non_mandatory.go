package usecases

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule/deletion"
)

// SkipNonMandatory is a use case that skips ModuleReleaseMetas that are not mandatory.
type SkipNonMandatory struct {
}

func NewSkipNonMandatory() *SkipNonMandatory {
	return &SkipNonMandatory{}
}

func (s *SkipNonMandatory) ShouldExecute(_ context.Context, mrm *v1beta2.ModuleReleaseMeta) (bool, error) {
	return mrm.Spec.Mandatory == nil || mrm.Spec.Mandatory.Version == "", nil
}

func (s *SkipNonMandatory) Execute(_ context.Context, _ *v1beta2.ModuleReleaseMeta) error {
	return deletion.ErrMrmNotMandatory
}
