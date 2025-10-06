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

type ModuleInfo struct {
	v1beta2.Module

	Enabled         bool
	ValidationError error
	Unmanaged       bool
}

func (a ModuleInfo) IsInstalledByVersion() bool {
	return a.configuredWithVersionInSpec() || a.installedwithVersionInStatus()
}

// configuredWithVersionInSpec returns true if the Module is enabled in Spec using
// a specific version instead of a channel.
func (a ModuleInfo) configuredWithVersionInSpec() bool {
	return a.Enabled && a.Version != "" && a.Channel == ""
}

// installedwithVersionInStatus returns true if the Module installed using a specific version
// (instead of a channel) is reported in Status.
func (a ModuleInfo) installedwithVersionInStatus() bool {
	return !a.Enabled && shared.NoneChannel.Equals(a.Channel) && a.Version != ""
}

// FetchModuleInfo returns a list of ModuleInfo objects containing information about modules referenced by the Kyma CR.
// This includes modules that are enabled in `.spec.modules[]` and modules that are not enabled
// in `.spec.modules[]` but still contain an entry in `.status.modules[]`.
func FetchModuleInfo(kyma *v1beta2.Kyma) []ModuleInfo {
	moduleMap := make(map[string]bool)
	modules := make([]ModuleInfo, 0)
	for _, module := range kyma.Spec.Modules {
		moduleMap[module.Name] = true
		if shared.NoneChannel.Equals(module.Channel) {
			modules = append(modules, ModuleInfo{
				Module:  module,
				Enabled: true,
				ValidationError: fmt.Errorf(
					"%w for module %s: Channel \"none\" is not allowed",
					ErrInvalidModuleInSpec,
					module.Name,
				),
				Unmanaged: !module.Managed,
			})
			continue
		}
		if module.Version != "" && module.Channel != "" {
			modules = append(modules, ModuleInfo{
				Module:  module,
				Enabled: true,
				ValidationError: fmt.Errorf(
					"%w for module %s: Version and channel are mutually exclusive options",
					ErrInvalidModuleInSpec,
					module.Name,
				),
				Unmanaged: !module.Managed,
			})
			continue
		}
		modules = append(modules, ModuleInfo{Module: module, Enabled: true, Unmanaged: !module.Managed})
	}

	for _, moduleInStatus := range kyma.Status.Modules {
		_, exist := moduleMap[moduleInStatus.Name]
		if exist {
			continue
		}

		modules = append(modules, ModuleInfo{
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
		return fmt.Errorf(
			"%w for module %s: ModuleTemplate reference is missing",
			ErrInvalidModuleInStatus,
			moduleStatus.Name,
		)
	}
	return nil
}
