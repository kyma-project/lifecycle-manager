package templatelookup

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

// GetMandatory returns ModuleTemplates TOs (Transfer Objects) which are marked are mandatory modules.
func GetMandatory(ctx context.Context, kymaClient client.Reader) (ModuleTemplatesByModuleName,
	error,
) {
	moduleTemplateList := &v1beta2.ModuleTemplateList{}
	if err := kymaClient.List(ctx, moduleTemplateList); err != nil {
		return nil, fmt.Errorf("could not list ModuleTemplates: %w", err)
	}

	mandatoryModules := make(map[string]*ModuleTemplateInfo)
	for _, moduleTemplate := range moduleTemplateList.Items {
		moduleTemplate := moduleTemplate
		if moduleTemplate.Spec.Mandatory && moduleTemplate.DeletionTimestamp.IsZero() {
			mandatoryModules[moduleTemplate.Name] = &ModuleTemplateInfo{
				ModuleTemplate: &moduleTemplate,
				Err:            nil,
			}
		}
	}
	return mandatoryModules, nil
}
