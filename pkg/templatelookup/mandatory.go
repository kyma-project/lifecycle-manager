package templatelookup

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	k8slabels "k8s.io/apimachinery/pkg/labels"
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

	mandatoryModules := make(map[string]*ModuleTemplateInfo)
	for _, moduleTemplate := range mandatoryModuleTemplateList.Items {
		if moduleTemplate.DeletionTimestamp.IsZero() {
			mandatoryModules[moduleTemplate.Name] = &ModuleTemplateInfo{
				ModuleTemplate: &moduleTemplate,
				Err:            nil,
			}
		}
	}
	return mandatoryModules, nil
}
