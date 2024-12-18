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

// configuredWithVersionInSpec returns true if the Module is enabled in Spec using a specific version instead of a channel.
func (a ModuleInfo) configuredWithVersionInSpec() bool {
	return a.Enabled && a.Version != "" && a.Channel == ""
}

// installedwithVersionInStatus returns true if the Module installed using a specific version (instead of a channel) is reported in Status.
func (a ModuleInfo) installedwithVersionInStatus() bool {
	return !a.Enabled && shared.NoneChannel.Equals(a.Channel) && a.Version != ""
}

// FetchModuleInfo returns a list of ModuleInfo objects containing information about modules referenced by the Kyma CR.
func FetchModuleInfo(kyma *v1beta2.Kyma) []ModuleInfo {
	moduleMap := buildModuleMap(kyma.Spec.Modules)
	specModules := buildModuleInfosFromSpec(kyma.Spec.Modules)
	statusModules := buildModuleInfosFromStatus(kyma.Status.Modules, moduleMap)

	return append(specModules, statusModules...)
}

// buildModuleMap creates a map for quick lookup of modules from Spec.Modules by name.
func buildModuleMap(modules []v1beta2.Module) map[string]struct{} {
	moduleMap := make(map[string]struct{}, len(modules))
	for _, module := range modules {
		moduleMap[module.Name] = struct{}{}
	}
	return moduleMap
}

// buildModuleInfosFromSpec processes Spec.Modules and returns a slice of ModuleInfo.
func buildModuleInfosFromSpec(modules []v1beta2.Module) []ModuleInfo {
	moduleInfos := make([]ModuleInfo, 0, len(modules))
	for _, module := range modules {
		validationError := validateModuleSpec(module)
		moduleInfos = append(moduleInfos, newEnabledModuleInfo(module, validationError))
	}
	return moduleInfos
}

// buildModuleInfosFromStatus processes Status.Modules and returns a slice of ModuleInfo.
func buildModuleInfosFromStatus(
	statusModules []v1beta2.ModuleStatus, moduleMap map[string]struct{},
) []ModuleInfo {
	moduleInfos := make([]ModuleInfo, 0, len(statusModules))
	for _, moduleStatus := range statusModules {
		if _, exists := moduleMap[moduleStatus.Name]; !exists {
			validationError := determineModuleValidity(moduleStatus)
			moduleInfos = append(moduleInfos, newDisabledModuleInfo(moduleStatus, validationError))
		}
	}
	return moduleInfos
}

// validateModuleSpec validates a module from Spec.Modules and returns an error if invalid.
func validateModuleSpec(module v1beta2.Module) error {
	if shared.NoneChannel.Equals(module.Channel) {
		return fmt.Errorf("%w for module %s: Channel \"none\" is not allowed", ErrInvalidModuleInSpec, module.Name)
	}
	if module.Version != "" && module.Channel != "" {
		return fmt.Errorf("%w for module %s: Version and channel are mutually exclusive options", ErrInvalidModuleInSpec, module.Name)
	}
	return nil
}

// determineModuleValidity validates a module from Status.Modules and returns an error if invalid.
func determineModuleValidity(moduleStatus v1beta2.ModuleStatus) error {
	if moduleStatus.Template == nil {
		return fmt.Errorf("%w for module %s: ModuleTemplate reference is missing", ErrInvalidModuleInStatus, moduleStatus.Name)
	}
	return nil
}

// newEnabledModuleInfo creates a ModuleInfo object for enabled modules.
func newEnabledModuleInfo(module v1beta2.Module, validationError error) ModuleInfo {
	return ModuleInfo{
		Module:          module,
		Enabled:         true,
		ValidationError: validationError,
		Unmanaged:       !module.Managed,
	}
}

// newDisabledModuleInfo creates a ModuleInfo object for disabled modules.
func newDisabledModuleInfo(moduleStatus v1beta2.ModuleStatus, validationError error) ModuleInfo {
	return ModuleInfo{
		Module: v1beta2.Module{
			Name:    moduleStatus.Name,
			Channel: moduleStatus.Channel,
			Version: moduleStatus.Version,
		},
		Enabled:         false,
		ValidationError: validationError,
	}
}
