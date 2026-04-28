package restrictedmodule

import (
	"context"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
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
	// nothing to do
	if len(d.restrictedDefaultModules) == 0 {
		return nil
	}

	numEnabledModules := len(kyma.Spec.Modules)

	// we first try to append all default modules and then update the Kyma if there are any changes.
	// failing to determine if a module should be defaulted or not should not cause the whole defaulting process to fail.
	for _, moduleName := range d.restrictedDefaultModules {
		log := logf.FromContext(ctx).WithValues("module", moduleName, "kyma", kyma.Name)

		if skipAlreadyEnabled(kyma, moduleName) {
			log.Info("Skipping defaulting as module is already enabled")
			continue
		}

		log.Info("Defaulting restricted module")

		mrm, err := d.moduleReleaseMetaRepo.Get(ctx, moduleName)
		if err != nil {
			log.Error(err, "Failed to get ModuleReleaseMeta")
			continue
		}

		match, err := d.matchFunc(mrm, kyma)
		if err != nil {
			log.Error(err, "Failed to match ModuleReleaseMeta")
			continue
		}

		if !match {
			continue
		}

		addModule(kyma, moduleName)
	}

	// only if updating the Kyma with the defaulted modules fails, we return an error.
	if numEnabledModules != len(kyma.Spec.Modules) {
		if err := d.kymaRepo.Update(ctx, kyma); err != nil {
			logf.FromContext(ctx).Error(err, "Failed to update Kyma with defaulted restricted module")
			return fmt.Errorf("Failed to update Kyma %s with defaulted restricted modules %s: %w",
				kyma.Name,
				d.restrictedDefaultModules,
				err,
			)
		}
	}

	return nil
}

func skipAlreadyEnabled(kyma *v1beta2.Kyma, moduleName string) bool {
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
