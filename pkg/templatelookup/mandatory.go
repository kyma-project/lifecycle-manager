package templatelookup

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

// GetMandatory returns ModuleTemplates TOs (Transfer Objects) which are marked are mandatory modules.
func GetMandatory(ctx context.Context, kymaClient client.Reader) (ModuleTemplatesByModuleName,
	error,
) {
	mandatoryModuleTemplateList := &v1beta2.ModuleTemplateList{}
	labelSelector := k8slabels.SelectorFromSet(k8slabels.Set{shared.IsMandatoryModule: shared.EnableLabelValue})
	if err := kymaClient.List(ctx, mandatoryModuleTemplateList,
		&client.ListOptions{LabelSelector: labelSelector}); err != nil {
		return nil, fmt.Errorf("could not list mandatory ModuleTemplates: %w", err)
	}

	// maps module name to the module template of the highest version encountered
	mandatoryModules := make(map[string]*ModuleTemplateInfo)
	for _, moduleTemplate := range mandatoryModuleTemplateList.Items {
		if moduleTemplate.DeletionTimestamp.IsZero() {
			currentModuleTemplate := &moduleTemplate
			moduleName := GetModuleName(currentModuleTemplate)
			if mandatoryModules[moduleName] != nil {
				var err error
				currentModuleTemplate, err = GetModuleTemplateWithHigherVersion(currentModuleTemplate,
					mandatoryModules[moduleName].ModuleTemplate)
				if err != nil {
					mandatoryModules[moduleName] = &ModuleTemplateInfo{
						ModuleTemplate: nil,
						Err:            err,
					}
					continue
				}
			}
			mandatoryModules[moduleName] = &ModuleTemplateInfo{
				ModuleTemplate: currentModuleTemplate,
				Err:            nil,
			}
		}
	}
	return mandatoryModules, nil
}

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

func GetModuleTemplateWithHigherVersion(
	firstModuleTemplate, secondModuleTemplate *v1beta2.ModuleTemplate,
) (*v1beta2.ModuleTemplate,
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
