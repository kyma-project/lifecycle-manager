package moduletemplateinfolookup

import (
	"context"
	"errors"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
)

var ErrNoResponsibleStrategy = errors.New("failed to find responsible module template lookup strategy")

type ModuleTemplateInfoLookupStrategy interface {
	Lookup(ctx context.Context,
		moduleInfo *templatelookup.ModuleInfo,
		kyma *v1beta2.Kyma,
		moduleReleaseMeta *v1beta2.ModuleReleaseMeta,
	) templatelookup.ModuleTemplateInfo
	IsResponsible(moduleInfo *templatelookup.ModuleInfo,
		kyma *v1beta2.Kyma,
		moduleReleaseMeta *v1beta2.ModuleReleaseMeta,
	) bool
}

type AggregatedModuleTemplateInfoLookupStrategy struct {
	strategies []ModuleTemplateInfoLookupStrategy
}

func NewAggregatedModuleTemplateInfoLookupStrategy(strategies []ModuleTemplateInfoLookupStrategy) AggregatedModuleTemplateInfoLookupStrategy {
	return AggregatedModuleTemplateInfoLookupStrategy{strategies: strategies}
}

func (s AggregatedModuleTemplateInfoLookupStrategy) Lookup(ctx context.Context,
	moduleInfo *templatelookup.ModuleInfo,
	kyma *v1beta2.Kyma,
	moduleReleaseMeta *v1beta2.ModuleReleaseMeta,
) templatelookup.ModuleTemplateInfo {
	for _, strategy := range s.strategies {
		if strategy.IsResponsible(moduleInfo, kyma, moduleReleaseMeta) {
			return strategy.Lookup(ctx, moduleInfo, kyma, moduleReleaseMeta)
		}
	}

	return templatelookup.ModuleTemplateInfo{
		Err: ErrNoResponsibleStrategy,
	}
}
