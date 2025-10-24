package installation

import (
	"context"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/errors/mandatorymodule/installation"
	modulecommon "github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
)

type ModuleReleaseMetaRepository interface {
	ListMandatory(ctx context.Context) ([]v1beta2.ModuleReleaseMeta, error)
}

type ModuleTemplateRepository interface {
	GetSpecificVersionForModule(ctx context.Context, moduleName, version string) (*v1beta2.ModuleTemplate, error)
}

type ModuleParser interface {
	GenerateMandatoryModulesFromTemplates(
		ctx context.Context,
		kyma *v1beta2.Kyma,
		templates templatelookup.ModuleTemplatesByModuleName,
	) modulecommon.Modules
}

type ManifestCreator interface {
	ReconcileManifests(
		ctx context.Context,
		kyma *v1beta2.Kyma,
		modules modulecommon.Modules,
	) error
}

type MandatoryModuleMetrics interface {
	RecordMandatoryModulesCount(count int)
}

type Service struct {
	mrmRepo ModuleReleaseMetaRepository
	mtRepo  ModuleTemplateRepository

	moduleParser           ModuleParser
	manifestCreator        ManifestCreator
	mandatoryModuleMetrics MandatoryModuleMetrics
}

func NewService(mrmRepo ModuleReleaseMetaRepository,
	mtRepo ModuleTemplateRepository,
	moduleParser ModuleParser,
	manifestCreator ManifestCreator,
	mandatoryModuleMetrics MandatoryModuleMetrics,
) *Service {
	return &Service{
		mrmRepo:                mrmRepo,
		mtRepo:                 mtRepo,
		moduleParser:           moduleParser,
		manifestCreator:        manifestCreator,
		mandatoryModuleMetrics: mandatoryModuleMetrics,
	}
}

func (s *Service) HandleInstallation(ctx context.Context, kyma *v1beta2.Kyma) error {
	if kyma.SkipReconciliation() {
		return installation.ErrSkippingReconciliationKyma
	}

	mandatoryMrms, err := s.mrmRepo.ListMandatory(ctx)
	if err != nil {
		return fmt.Errorf("list mandatory modules failed: %w", err)
	}
	s.mandatoryModuleMetrics.RecordMandatoryModulesCount(len(mandatoryMrms))

	var mandatoryTemplatesByName = make(templatelookup.ModuleTemplatesByModuleName)
	for _, mrm := range mandatoryMrms {
		moduleTemplate, err := s.mtRepo.GetSpecificVersionForModule(ctx, mrm.Name, mrm.Spec.Mandatory.Version)
		if err != nil {
			return fmt.Errorf("get ModuleTemplate for mandatory module %s failed: %w", mrm.Name, err)
		}
		// TODO: Append ocm identity
		mandatoryTemplatesByName[moduleTemplate.Spec.ModuleName] = &templatelookup.ModuleTemplateInfo{
			ModuleTemplate: moduleTemplate,
			Err:            nil,
		}
	}
	modules := s.moduleParser.GenerateMandatoryModulesFromTemplates(ctx, kyma, mandatoryTemplatesByName)
	if err := s.manifestCreator.ReconcileManifests(ctx, kyma, modules); err != nil {
		return fmt.Errorf("reconcile manifests for mandatory modules failed: %w", err)
	}

	return nil
}
