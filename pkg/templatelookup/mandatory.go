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
			moduleName := moduleTemplate.Spec.ModuleName
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

func getModuleSemverVersion(moduleTemplate *v1beta2.ModuleTemplate) (*semver.Version, error) {
	if moduleTemplate.Spec.Version == "" {
		return nil, fmt.Errorf("module template %s does not have a version", moduleTemplate.Name)
	}

	version, err := semver.NewVersion(moduleTemplate.Spec.Version)
	if err != nil {
		return nil, fmt.Errorf("could not parse version as a semver: %s: %w",
			moduleTemplate.Spec.Version, err)
	}
	return version, nil

}

func GetModuleTemplateWithHigherVersion(first, second *v1beta2.ModuleTemplate) (*v1beta2.ModuleTemplate, error) {
	firstVersion, err := getModuleSemverVersion(first)
	if err != nil {
		return nil, err
	}

	secondVersion, err := getModuleSemverVersion(second)
	if err != nil {
		return nil, err
	}

	if firstVersion.GreaterThan(secondVersion) {
		return first, nil
	}

	return second, nil
}
