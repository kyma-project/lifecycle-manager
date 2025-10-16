package usecases

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type MrmEnsureFinalizerRepo interface {
	EnsureFinalizer(ctx context.Context, moduleName string, finalizer string) error
}

// EnsureFinalizer is responsible for ensuring that the mandatory finalizer is present on the ModuleReleaseMeta.
type EnsureFinalizer struct {
	repo MrmEnsureFinalizerRepo
}

func NewEnsureFinalizer(repo MrmEnsureFinalizerRepo) *EnsureFinalizer {
	return &EnsureFinalizer{repo: repo}
}

func (e *EnsureFinalizer) IsApplicable(_ context.Context, mrm *v1beta2.ModuleReleaseMeta) (bool, error) {
	return !controllerutil.ContainsFinalizer(mrm, shared.MandatoryModuleFinalizer), nil
}

func (e *EnsureFinalizer) Execute(ctx context.Context, mrm *v1beta2.ModuleReleaseMeta) error {
	return e.repo.EnsureFinalizer(ctx, mrm.Name, shared.MandatoryModuleFinalizer)
}
