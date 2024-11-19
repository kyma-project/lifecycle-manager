package templatelookup

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
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
	fieldSelector := fields.SelectorFromSet(fields.Set{"metadata.deletionTimestamp": ""})
	listOptions := &client.ListOptions{LabelSelector: labelSelector, FieldSelector: fieldSelector}
	if err := kymaClient.List(ctx, mandatoryModuleTemplateList, listOptions); err != nil {
		return nil, fmt.Errorf("could not list mandatory ModuleTemplates: %w", err)
	}

	mandatoryModules := make(map[string]*ModuleTemplateInfo)
	for _, moduleTemplate := range mandatoryModuleTemplateList.Items {
		mandatoryModules[moduleTemplate.Name] = &ModuleTemplateInfo{
			ModuleTemplate: &moduleTemplate,
			Err:            nil,
		}
	}
	return mandatoryModules, nil
}
