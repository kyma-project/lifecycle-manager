package restrictedmodule

import (
	"context"
	"fmt"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type ModuleReleaseMetaRepository interface {
	Get(ctx context.Context, moduleName string) (*v1beta2.ModuleReleaseMeta, error)
}

type KymaRepository interface {
	Update(ctx context.Context, kyma *v1beta2.Kyma) error
}

type RestrictedModuleMatchFunc = func(mrm *v1beta2.ModuleReleaseMeta, kyma *v1beta2.Kyma) (bool, error)

type Defaulter struct {
	restrictedDefaultModules []string
	moduleReleaseMetaRepo    ModuleReleaseMetaRepository
	kymaRepo                 KymaRepository
	matchFunc                RestrictedModuleMatchFunc
}

func NewDefaulter(restrictedDefaultModules []string,
	moduleReleaseMetaRepo ModuleReleaseMetaRepository,
	kymaRepo KymaRepository,
	matchFunc RestrictedModuleMatchFunc,
) *Defaulter {
	return &Defaulter{
		restrictedDefaultModules: restrictedDefaultModules,
		moduleReleaseMetaRepo:    moduleReleaseMetaRepo,
		kymaRepo:                 kymaRepo,
		matchFunc:                matchFunc,
	}
}

// Default adds restricted default modules to Kyma if they are not already enabled and
// if the kymaSelector defined in the module's ModuleReleaseMeta matches the provided Kyma.
func (d *Defaulter) Default(ctx context.Context, kyma *v1beta2.Kyma) error {
	log := logf.FromContext(ctx).WithValues(
		"kyma", kyma.Name,
		"service", "restricted module defaulter",
	)

	if !kyma.GetDeletionTimestamp().IsZero() {
		return nil
	}

	if len(d.restrictedDefaultModules) == 0 {
		return nil
	}

	alreadyDefaultedModules := len(kyma.Spec.Modules)

	// First try to append all default modules and then update the Kyma if there are any changes.
	// failing to determine if a module should be defaulted or not should not cause the whole defaulting process to fail
	for _, moduleName := range d.restrictedDefaultModules {
		moduleLog := log.WithValues("module", moduleName)

		if isAlreadyEnabled(kyma, moduleName) {
			continue
		}

		mrm, err := d.moduleReleaseMetaRepo.Get(ctx, moduleName)
		if err != nil {
			moduleLog.Error(err, "Failed to get ModuleReleaseMeta")
			continue
		}

		match, err := d.matchFunc(mrm, kyma)
		if err != nil {
			moduleLog.Error(err, "Failed to get Kyma selector from ModuleReleaseMeta")
			continue
		}

		if !match {
			continue
		}

		moduleLog.Info("Adding restricted default module to Kyma spec")
		addModule(kyma, moduleName)
	}

	// nothing updated
	if alreadyDefaultedModules == len(kyma.Spec.Modules) {
		return nil
	}

	// only if updating the Kyma with the restricted default modules fails, we return an error.
	if err := d.kymaRepo.Update(ctx, kyma); err != nil {
		log.Error(err, "Failed to update Kyma")
		return fmt.Errorf("failed to update Kyma %s with restricted default modules: %w",
			kyma.Name,
			err,
		)
	}

	return nil
}

func isAlreadyEnabled(kyma *v1beta2.Kyma, moduleName string) bool {
	for _, module := range kyma.Spec.Modules {
		if module.Name == moduleName {
			return true
		}
	}
	return false
}

func addModule(kyma *v1beta2.Kyma, moduleName string) {
	kyma.Spec.Modules = append(kyma.Spec.Modules, v1beta2.Module{
		Name:                 moduleName,
		CustomResourcePolicy: v1beta2.CustomResourcePolicyCreateAndDelete,
		Managed:              true,
	})
}
