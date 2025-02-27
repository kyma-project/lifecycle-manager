package service

import (
	"context"
	"fmt"

	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/repository"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
)

type MandatoryModuleService struct {
	moduleTemplateRepository *repository.ModuleTemplateRepository
}

func NewMandatoryModuleService(client client.Client) *MandatoryModuleService {
	return &MandatoryModuleService{
		moduleTemplateRepository: repository.NewModuleTemplateRepository(client),
	}
}

// GetMandatory returns ModuleTemplates TOs (Transfer Objects) which are marked are mandatory modules.
func (m *MandatoryModuleService) GetMandatory(ctx context.Context) (templatelookup.ModuleTemplatesByModuleName,
	error,
) {
	mandatoryModuleTemplateList, err := m.moduleTemplateRepository.ListByLabel(ctx,
		k8slabels.SelectorFromSet(k8slabels.Set{shared.IsMandatoryModule: "true"}))
	if err != nil {
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
