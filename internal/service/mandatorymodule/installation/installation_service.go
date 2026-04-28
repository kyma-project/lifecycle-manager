package installation

import (
	"context"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
	installerrors "github.com/kyma-project/lifecycle-manager/internal/errors/mandatorymodule/installation"
	modulecommon "github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	if !kyma.DeletionTimestamp.IsZero() {
		return installerrors.ErrKymaBeingDeleted
	}

	if kyma.SkipReconciliation() {
		return installerrors.ErrSkipReconcileKyma
	}

	mandatoryMrms, err := s.mrmRepo.ListMandatory(ctx)
	if err != nil {
		return fmt.Errorf("list mandatory modules failed: %w", err)
	}

	s.mandatoryModuleMetrics.RecordMandatoryModulesCount(len(mandatoryMrms))
	mandatoryTemplatesByName := make(templatelookup.ModuleTemplatesByModuleName)
	matchesKyma := make(map[string]bool)
	for _, mrm := range mandatoryMrms {

		if !mrm.DeletionTimestamp.IsZero() {
			continue
		}
		moduleTemplate, err := s.mtRepo.GetSpecificVersionForModule(ctx, mrm.Name, mrm.Spec.Mandatory.Version)
		if err != nil {
			return fmt.Errorf("get ModuleTemplate for mandatory module %s failed: %w", mrm.Name, err)
		}
		ocmId, err := ocmidentity.NewComponentId(mrm.Spec.OcmComponentName, moduleTemplate.Spec.Version)
		if err != nil {
			err = fmt.Errorf("failed creating OCM identity for module %s in namespace %s: %w",
				moduleTemplate.Spec.ModuleName, moduleTemplate.Namespace, err)
			mandatoryTemplatesByName[moduleTemplate.Spec.ModuleName] = createMandatoryModuleTemplateInfo(moduleTemplate,
				err,
				nil)
			continue
		}
		mandatoryTemplatesByName[moduleTemplate.Spec.ModuleName] = createMandatoryModuleTemplateInfo(moduleTemplate,
			nil, ocmId)

		if mrm.Spec.KymaLabelSelector == nil {
			// nil selector means match all Kymas
			matchesKyma[moduleTemplate.Spec.ModuleName] = true
		} else {
			selector, err := apimetav1.LabelSelectorAsSelector(mrm.Spec.KymaLabelSelector)
			if err != nil {
				continue
			}
			matchesKyma[moduleTemplate.Spec.ModuleName] = selector.Matches(labels.Set(kyma.ObjectMeta.Labels))
		}
	}
	modules := s.moduleParser.GenerateMandatoryModulesFromTemplates(ctx, kyma, mandatoryTemplatesByName)

	for _, module := range modules {
		if !matchesKyma[module.TemplateInfo.ModuleTemplate.Spec.ModuleName] {
			module.TemplateInfo.Err = templatelookup.ErrTemplateNotAllowed
		}
	}

	if err := s.manifestCreator.ReconcileManifests(ctx, kyma, modules); err != nil {
		return fmt.Errorf("reconcile manifests for mandatory modules failed: %w", err)
	}

	return nil
}

func createMandatoryModuleTemplateInfo(template *v1beta2.ModuleTemplate,
	err error,
	componentId *ocmidentity.ComponentId,
) *templatelookup.ModuleTemplateInfo {
	template.Spec.Mandatory = true // hotfix to ensure Mandatory field is set
	return &templatelookup.ModuleTemplateInfo{
		ModuleTemplate: template,
		Err:            err,
		ComponentId:    componentId,
	}
}
