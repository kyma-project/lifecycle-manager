package templatelookup

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func GetModuleName(moduleTemplate *v1beta2.ModuleTemplate) string {
	if moduleTemplate.Spec.ModuleName != "" {
		return moduleTemplate.Spec.ModuleName
	}

	// https://github.com/kyma-project/lifecycle-manager/issues/2135
	// Remove this after warden ModuleTemplate is created using modulectl
	return moduleTemplate.Labels[shared.ModuleName]
}

func GetModuleSemverVersion(moduleTemplate *v1beta2.ModuleTemplate) (*semver.Version, error) {
	if moduleTemplate.Spec.Version != "" {
		version, err := semver.NewVersion(moduleTemplate.Spec.Version)
		if err != nil {
			return nil, fmt.Errorf("could not parse version as a semver: %s: %w",
				moduleTemplate.Spec.Version, err)
		}
		return version, nil
	}

	// https://github.com/kyma-project/lifecycle-manager/issues/2135
	// Remove this after warden ModuleTemplate is created using modulectl
	version, err := semver.NewVersion(moduleTemplate.Annotations[shared.ModuleVersionAnnotation])
	if err != nil {
		return nil, fmt.Errorf("could not parse version as a semver %s: %w",
			moduleTemplate.Annotations[shared.ModuleVersionAnnotation], err)
	}
	return version, nil
}

func GetModuleTemplateWithHigherVersion(firstModuleTemplate, secondModuleTemplate *v1beta2.ModuleTemplate) (*v1beta2.ModuleTemplate,
	error,
) {
	firstVersion, err := GetModuleSemverVersion(firstModuleTemplate)
	if err != nil {
		return nil, err
	}

	secondVersion, err := GetModuleSemverVersion(secondModuleTemplate)
	if err != nil {
		return nil, err
	}

	if firstVersion.GreaterThan(secondVersion) {
		return firstModuleTemplate, nil
	}

	return secondModuleTemplate, nil
}
