package usecases

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/event"
)

const SettingFinalizerErrorEvent event.Reason = "SettingMandatoryModuleTemplateFinalizerError"

type MrmEnsureFinalizerRepo interface {
	EnsureFinalizer(ctx context.Context, moduleName string, finalizer string) error
}

// EnsureFinalizer is responsible for ensuring that the mandatory finalizer is present on the ModuleReleaseMeta.
type EnsureFinalizer struct {
	repo         MrmEnsureFinalizerRepo
	eventHandler EventHandler
}

func NewEnsureFinalizer(repo MrmEnsureFinalizerRepo, eventHandler EventHandler) *EnsureFinalizer {
	return &EnsureFinalizer{repo: repo, eventHandler: eventHandler}
}

// IsApplicable returns true if the ModuleReleaseMeta does not contain the mandatory finalizer, so it should be added.
func (e *EnsureFinalizer) IsApplicable(_ context.Context, mrm *v1beta2.ModuleReleaseMeta) (bool, error) {
	return !controllerutil.ContainsFinalizer(mrm, shared.MandatoryModuleFinalizer), nil
}

func (e *EnsureFinalizer) Execute(ctx context.Context, mrm *v1beta2.ModuleReleaseMeta) error {
	if err := e.repo.EnsureFinalizer(ctx, mrm.Name, shared.MandatoryModuleFinalizer); err != nil {
		e.eventHandler.Warning(mrm, SettingFinalizerErrorEvent, err)
		return err
	}
	return nil
}
