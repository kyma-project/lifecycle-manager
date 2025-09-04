package templatelookup

import (
	"context"
	"fmt"

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
			moduleName := currentModuleTemplate.GetModuleName()
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

func GetModuleTemplateWithHigherVersion(firstModuleTemplate, secondModuleTemplate *v1beta2.ModuleTemplate) (*v1beta2.ModuleTemplate,
	error,
) {
	firstVersion, err := firstModuleTemplate.GetSemanticVersion()
	if err != nil {
		return nil, err
	}

	secondVersion, err := secondModuleTemplate.GetSemanticVersion()
	if err != nil {
		return nil, err
	}

	if firstVersion.GreaterThan(secondVersion) {
		return firstModuleTemplate, nil
	}

	return secondModuleTemplate, nil
}
