package templatelookup

import (
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

var (
	ErrInvalidModuleInSpec   = errors.New("invalid configuration in Kyma spec.modules")
	ErrInvalidModuleInStatus = errors.New("invalid module entry in Kyma status")
)

type AvailableModule struct {
	v1beta2.Module
	Enabled         bool
	ValidationError error
}

func (a AvailableModule) IsInstalledByVersion() bool {
	return a.configuredWithVersionInSpec() || a.installedwithVersionInStatus()
}

// configuredWithVersionInSpec returns true if the Module is enabled in Spec using a specific version instead of a channel.
func (a AvailableModule) configuredWithVersionInSpec() bool {
	return a.Enabled && a.Version != "" && a.Channel == ""
}

// installedwithVersionInStatus returns true if the Module installed using a specific version (instead of a channel) is reported in Status.
func (a AvailableModule) installedwithVersionInStatus() bool {
	return !a.Enabled && shared.NoneChannel.Equals(a.Channel) && a.Version != ""
}

// FindAvailableModules returns a list of AvailableModule objects based on the Kyma CR Spec and Status.
func FindAvailableModules(kyma *v1beta2.Kyma) []AvailableModule {
	moduleMap := make(map[string]bool)
	modules := make([]AvailableModule, 0)
	for _, module := range kyma.Spec.Modules {
		moduleMap[module.Name] = true
		if shared.NoneChannel.Equals(module.Channel) {
			modules = append(modules, AvailableModule{Module: module, Enabled: true, ValidationError: fmt.Errorf("%w for module %s: Channel \"none\" is not allowed", ErrInvalidModuleInSpec, module.Name)})
			continue
		}
		if module.Version != "" && module.Channel != "" {
			modules = append(modules, AvailableModule{Module: module, Enabled: true, ValidationError: fmt.Errorf("%w for module %s: Version and channel are mutually exclusive options", ErrInvalidModuleInSpec, module.Name)})
			continue
		}
		modules = append(modules, AvailableModule{Module: module, Enabled: true})
	}

	for _, moduleInStatus := range kyma.Status.Modules {
		_, exist := moduleMap[moduleInStatus.Name]
		if exist {
			continue
		}

		modules = append(modules, AvailableModule{
			Module: v1beta2.Module{
				Name:    moduleInStatus.Name,
				Channel: moduleInStatus.Channel,
				Version: moduleInStatus.Version,
			},
			Enabled:         false,
			ValidationError: determineModuleValidity(moduleInStatus),
		})
	}
	return modules
}

func determineModuleValidity(moduleStatus v1beta2.ModuleStatus) error {
	if moduleStatus.Template == nil {
		return fmt.Errorf("%w for module %s: ModuleTemplate reference is missing", ErrInvalidModuleInStatus, moduleStatus.Name)
	}
	return nil
}
