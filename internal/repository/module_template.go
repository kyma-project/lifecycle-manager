package repository

import (
	"context"
	"fmt"

	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
)

type ModuleTemplateRepository struct {
	client.Client
}

func NewModuleTemplateRepository(client client.Client) *ModuleTemplateRepository {
	return &ModuleTemplateRepository{Client: client}
}

func (m *ModuleTemplateRepository) Get(ctx context.Context,
	namespacedName client.ObjectKey,
) (*v1beta2.ModuleTemplate, error) {
	template := &v1beta2.ModuleTemplate{}
	if err := m.Client.Get(ctx, namespacedName, template); err != nil {
		return nil, fmt.Errorf("failed to get ModuleTemplate: %w", err)
	}
	return template, nil
}

func (m *ModuleTemplateRepository) Update(ctx context.Context,
	template *v1beta2.ModuleTemplate,
) error {
	if err := m.Update(ctx, template); err != nil {
		return fmt.Errorf("failed to update ModuleTemplate: %w", err)
	}
	return nil
}

// GetMandatory returns ModuleTemplates TOs (Transfer Objects) which are marked are mandatory modules.
func (m *ModuleTemplateRepository) GetMandatory(ctx context.Context) (templatelookup.ModuleTemplatesByModuleName,
	error,
) {
	mandatoryModuleTemplateList := &v1beta2.ModuleTemplateList{}
	labelSelector := k8slabels.SelectorFromSet(k8slabels.Set{shared.IsMandatoryModule: shared.EnableLabelValue})
	if err := m.Client.List(ctx, mandatoryModuleTemplateList,
		&client.ListOptions{LabelSelector: labelSelector}); err != nil {
		return nil, fmt.Errorf("could not list mandatory ModuleTemplates: %w", err)
	}

	// maps module name to the module template of the highest version encountered
	mandatoryModules := make(map[string]*templatelookup.ModuleTemplateInfo)
	for _, moduleTemplate := range mandatoryModuleTemplateList.Items {
		if moduleTemplate.DeletionTimestamp.IsZero() {
			currentModuleTemplate := &moduleTemplate
			moduleName := templatelookup.GetModuleName(currentModuleTemplate)
			if mandatoryModules[moduleName] != nil {
				var err error
				currentModuleTemplate, err = templatelookup.GetModuleTemplateWithHigherVersion(currentModuleTemplate,
					mandatoryModules[moduleName].ModuleTemplate)
				if err != nil {
					mandatoryModules[moduleName] = &templatelookup.ModuleTemplateInfo{
						ModuleTemplate: nil,
						Err:            err,
					}
					continue
				}
			}
			mandatoryModules[moduleName] = &templatelookup.ModuleTemplateInfo{
				ModuleTemplate: currentModuleTemplate,
				Err:            nil,
			}
		}
	}
	return mandatoryModules, nil
}
