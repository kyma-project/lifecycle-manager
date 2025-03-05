package service

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/parser"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/repository"
	"github.com/kyma-project/lifecycle-manager/pkg/module/sync"
)

type MandatoryModuleInstallationService struct {
	moduleTemplateRepository *repository.ModuleTemplateRepository
	mandatoryModuleService   *MandatoryModuleService
	metrics                  *metrics.MandatoryModulesMetrics
	runner                   *sync.Runner
	parser                   *parser.Parser
}

func NewMandatoryModuleInstallationService(client client.Client, metrics *metrics.MandatoryModulesMetrics,
	parser *parser.Parser,
) *MandatoryModuleInstallationService {
	return &MandatoryModuleInstallationService{
		moduleTemplateRepository: repository.NewModuleTemplateRepository(client),
		mandatoryModuleService:   NewMandatoryModuleService(client),
		metrics:                  metrics,
		runner:                   sync.New(client),
		parser:                   parser,
	}
}

func (s *MandatoryModuleInstallationService) ReconcileMandatoryModules(ctx context.Context, kyma *v1beta2.Kyma) error {
	mandatoryTemplates, err := s.mandatoryModuleService.GetMandatory(ctx)
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
