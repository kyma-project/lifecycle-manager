package service

import (
	"context"
	"fmt"

	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/parser"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/repository"
	"github.com/kyma-project/lifecycle-manager/pkg/module/sync"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
)

type MandatoryModuleInstallationService struct {
	moduleTemplateRepository *repository.ModuleTemplateRepository
	metrics                  *metrics.MandatoryModulesMetrics
	runner                   *sync.Runner
	parser                   *parser.Parser
}

func NewMandatoryModuleInstallationService(client client.Client, metrics *metrics.MandatoryModulesMetrics,
	parser *parser.Parser,
) *MandatoryModuleInstallationService {
	return &MandatoryModuleInstallationService{
		moduleTemplateRepository: repository.NewModuleTemplateRepository(client),
		metrics:                  metrics,
		runner:                   sync.New(client),
		parser:                   parser,
	}
}

func (s *MandatoryModuleInstallationService) ReconcileMandatoryModules(ctx context.Context, kyma *v1beta2.Kyma) error {
	mandatoryTemplates, err := s.GetMandatory(ctx)
	if err != nil {
		return fmt.Errorf("failed to get mandatory templates: %w", err)
	}
	s.metrics.RecordMandatoryTemplatesCount(len(mandatoryTemplates))

	modules := s.parser.GenerateMandatoryModulesFromTemplates(ctx, kyma, mandatoryTemplates)
	if err := s.runner.ReconcileManifests(ctx, kyma, modules); err != nil {
		return fmt.Errorf("failed to reconcile manifests: %w", err)
	}

	return nil
}

// GetMandatory returns ModuleTemplates TOs (Transfer Objects) which are marked are mandatory modules.
func (m *MandatoryModuleInstallationService) GetMandatory(ctx context.Context) (templatelookup.ModuleTemplatesByModuleName,
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
