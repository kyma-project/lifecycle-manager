package moduletemplateinfolookup

import (
	"context"
	"errors"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
)

var ErrNoResponsibleStrategy = errors.New("failed to find responsible module template lookup strategy")

type ModuleTemplateInfoLookupStrategy interface {
	// IsResponsible checks if the strategy is responsible for the given module installation.
	IsResponsible(moduleInfo *templatelookup.ModuleInfo,
		moduleReleaseMeta *v1beta2.ModuleReleaseMeta,
	) bool
	// Lookup looks up the required module template.
	Lookup(ctx context.Context,
		moduleInfo *templatelookup.ModuleInfo,
		kyma *v1beta2.Kyma,
		moduleReleaseMeta *v1beta2.ModuleReleaseMeta,
	) templatelookup.ModuleTemplateInfo
}

// ModuleTemplateInfoLookupStrategies is a strategy that aggregates multiple ModuleTemplateInfoLookupStrategies.
// It iterates over the strategies and uses the first one that is responsible for the given module info.
type ModuleTemplateInfoLookupStrategies struct {
	strategies []ModuleTemplateInfoLookupStrategy
}

func NewModuleTemplateInfoLookupStrategies(
	strategies []ModuleTemplateInfoLookupStrategy,
) ModuleTemplateInfoLookupStrategies {
	return ModuleTemplateInfoLookupStrategies{strategies: strategies}
}

func (s ModuleTemplateInfoLookupStrategies) Lookup(ctx context.Context,
	moduleInfo *templatelookup.ModuleInfo,
	kyma *v1beta2.Kyma,
	moduleReleaseMeta *v1beta2.ModuleReleaseMeta,
) templatelookup.ModuleTemplateInfo {
	for _, strategy := range s.strategies {
		if strategy.IsResponsible(moduleInfo, moduleReleaseMeta) {
			return strategy.Lookup(ctx, moduleInfo, kyma, moduleReleaseMeta)
		}
	}

	return templatelookup.ModuleTemplateInfo{
		Err: ErrNoResponsibleStrategy,
	}
}
