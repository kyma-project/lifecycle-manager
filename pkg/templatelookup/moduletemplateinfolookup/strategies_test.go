package moduletemplateinfolookup_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
)

func Test_ModuleTemplateInfoLookupStrategies_Lookup_CallsResponsibleStrategy(t *testing.T) {
	nonResponsibleStrategy := newLookupStrategyStub(false)
	responsibleStrategy := newLookupStrategyStub(true)
	strategies := moduletemplateinfolookup.NewModuleTemplateInfoLookupStrategies(
		[]moduletemplateinfolookup.ModuleTemplateInfoLookupStrategy{
			&nonResponsibleStrategy,
			&responsibleStrategy,
		},
	)

	moduleTemplateInfo := strategies.Lookup(t.Context(), nil, nil, nil)

	assert.True(t, responsibleStrategy.called)
	assert.False(t, nonResponsibleStrategy.called)
	require.NoError(t, moduleTemplateInfo.Err)
}

func Test_ModuleTemplateInfoLookupStrategies_Lookup_CallsFirstResponsibleStrategy(t *testing.T) {
	nonResponsibleStrategy := newLookupStrategyStub(false)
	responsibleStrategy := newLookupStrategyStub(true)
	responsibleStrategy2 := newLookupStrategyStub(true)
	strategies := moduletemplateinfolookup.NewModuleTemplateInfoLookupStrategies(
		[]moduletemplateinfolookup.ModuleTemplateInfoLookupStrategy{
			&nonResponsibleStrategy,
			&responsibleStrategy,
			&responsibleStrategy2,
		},
	)

	moduleTemplateInfo := strategies.Lookup(t.Context(), nil, nil, nil)

	assert.True(t, responsibleStrategy.called)
	assert.False(t, responsibleStrategy2.called)
	assert.False(t, nonResponsibleStrategy.called)
	require.NoError(t, moduleTemplateInfo.Err)
}

func Test_ModuleTemplateInfoLookupStrategies_Lookup_ReturnsFailureWhenNoStrategyResponsible(t *testing.T) {
	nonResponsibleStrategy := newLookupStrategyStub(false)
	strategies := moduletemplateinfolookup.NewModuleTemplateInfoLookupStrategies(
		[]moduletemplateinfolookup.ModuleTemplateInfoLookupStrategy{
			&nonResponsibleStrategy,
		},
	)

	moduleTemplateInfo := strategies.Lookup(t.Context(), nil, nil, nil)

	assert.False(t, nonResponsibleStrategy.called)
	require.ErrorIs(t, moduleTemplateInfo.Err, moduletemplateinfolookup.ErrNoResponsibleStrategy)
}

func Test_ModuleTemplateInfoLookupStrategies_Lookup_ReturnsFailureWhenNoStrategies(t *testing.T) {
	strategies := moduletemplateinfolookup.NewModuleTemplateInfoLookupStrategies(
		[]moduletemplateinfolookup.ModuleTemplateInfoLookupStrategy{},
	)

	moduleTemplateInfo := strategies.Lookup(t.Context(), nil, nil, nil)

	require.ErrorIs(t, moduleTemplateInfo.Err, moduletemplateinfolookup.ErrNoResponsibleStrategy)
}

func newLookupStrategyStub(responsible bool) LookupStrategyStub {
	return LookupStrategyStub{
		responsible: responsible,
	}
}

type LookupStrategyStub struct {
	responsible bool
	called      bool
}

func (s *LookupStrategyStub) Lookup(ctx context.Context,
	_ *templatelookup.ModuleInfo,
	_ *v1beta2.Kyma,
	_ *v1beta2.ModuleReleaseMeta,
) templatelookup.ModuleTemplateInfo {
	s.called = true
	return templatelookup.ModuleTemplateInfo{}
}

func (s *LookupStrategyStub) IsResponsible(_ *templatelookup.ModuleInfo, _ *v1beta2.ModuleReleaseMeta,
) bool {
	return s.responsible
}
