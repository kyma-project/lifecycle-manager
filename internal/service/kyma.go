package service

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/parser"
	"github.com/kyma-project/lifecycle-manager/internal/repository"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/module/sync"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
)

type KymaService struct {
	Client              client.Client
	KymaRepository      *repository.KymaRepository
	DescriptorProvider  *provider.CachedDescriptorProvider
	RequeueIntervals    queue.RequeueIntervals
	RemoteSyncNamespace string
	InKCPMode           bool
	Metrics             *metrics.MandatoryModulesMetrics
}

func (s *KymaService) GetKyma(ctx context.Context, namespacedName client.ObjectKey) (*v1beta2.Kyma, error) {
	return s.KymaRepository.GetKyma(ctx, namespacedName)
}

func (s *KymaService) ReconcileMandatoryModules(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	mandatoryTemplates, err := templatelookup.GetMandatory(ctx, s.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get mandatory templates: %w", err)
	}
	s.Metrics.RecordMandatoryTemplatesCount(len(mandatoryTemplates))

	modules, err := s.GenerateModulesFromTemplate(ctx, mandatoryTemplates, kyma)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to generate modules from template: %w", err)
	}

	runner := sync.New(s)
	if err := runner.ReconcileManifests(ctx, kyma, modules); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile manifests: %w", err)
	}

	return ctrl.Result{}, nil
}

func (s *KymaService) GenerateModulesFromTemplate(ctx context.Context,
	templates templatelookup.ModuleTemplatesByModuleName, kyma *v1beta2.Kyma) (common.Modules, error) {
	parser := parser.NewParser(s.Client, s.DescriptorProvider, s.InKCPMode, s.RemoteSyncNamespace)
	return parser.GenerateMandatoryModulesFromTemplates(ctx, kyma, templates), nil
}
