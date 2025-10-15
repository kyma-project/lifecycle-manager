package usecases

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type MrmRemoFinalizerRepo interface {
	RemoveFinalizer(ctx context.Context, moduleName string, finalizer string) error
}

// RemoveFinalizer is responsible for removing the mandatory finalizer from the ModuleReleaseMeta.
type RemoveFinalizer struct {
	repo MrmRemoFinalizerRepo
}

func NewRemoveFinalizer(repo MrmRemoFinalizerRepo) *RemoveFinalizer {
	return &RemoveFinalizer{repo: repo}
}

func (e *RemoveFinalizer) ShouldExecute(_ context.Context, mrm *v1beta2.ModuleReleaseMeta) (bool, error) {
	return controllerutil.ContainsFinalizer(mrm, shared.MandatoryModuleFinalizer), nil
}

func (e *RemoveFinalizer) Execute(ctx context.Context, mrm *v1beta2.ModuleReleaseMeta) error {
	return e.repo.RemoveFinalizer(ctx, mrm.Name, shared.MandatoryModuleFinalizer)
}
