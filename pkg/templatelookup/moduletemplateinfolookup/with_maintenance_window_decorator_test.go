package moduletemplateinfolookup_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
)

func Test_WithMWDecorator_IsResponsible_CallsDecoratedIsResponsible(t *testing.T) {
	decorated := &lookupStrategyStub{
		responsible: true,
	}
	withMaintenanceWindowDecorator := moduletemplateinfolookup.NewWithMaintenanceWindowDecorator(nil, decorated)

	responsible := withMaintenanceWindowDecorator.IsResponsible(nil, nil)

	assert.True(t, decorated.called)
	assert.True(t, responsible)
}

func Test_WithMWDecorator_Lookup_ReturnsModuleTemplateInfo_WhenDecoratedLookupReturnsModuleTemplateInfoWithError(
	t *testing.T,
) {
	maintenanceWindow := &maintenanceWindowStub{}
	expectedModuleTemplateInfo := templatelookup.ModuleTemplateInfo{
		Err: errors.New("test error"),
	}
	decorated := &lookupStrategyStub{
		moduleTemplateInfo: expectedModuleTemplateInfo,
	}
	withMaintenanceWindowDecorator := moduletemplateinfolookup.NewWithMaintenanceWindowDecorator(maintenanceWindow,
		decorated)

	moduleTemplateInfo := withMaintenanceWindowDecorator.Lookup(t.Context(),
		nil,
		nil,
		nil)

	assert.False(t, maintenanceWindow.requiredCalled)
	assert.False(t, maintenanceWindow.activeCalled)
	assert.Equal(t, expectedModuleTemplateInfo, moduleTemplateInfo)
}

func Test_WithMWDecorator_Lookup_ReturnsModuleTemplateInfo_WhenDecoratedLookupReturnsNilTemplate(t *testing.T) {
	maintenanceWindow := &maintenanceWindowStub{}
	decorated := &lookupStrategyStub{
		moduleTemplateInfo: templatelookup.ModuleTemplateInfo{
			ModuleTemplate: nil,
		},
	}
	withMaintenanceWindowDecorator := moduletemplateinfolookup.NewWithMaintenanceWindowDecorator(maintenanceWindow,
		decorated)

	moduleTemplateInfo := withMaintenanceWindowDecorator.Lookup(t.Context(),
		nil,
		nil,
		nil)

	assert.False(t, maintenanceWindow.requiredCalled)
	assert.False(t, maintenanceWindow.activeCalled)
	assert.Nil(t, moduleTemplateInfo.ModuleTemplate)
}

func Test_WithMWDecorator_Lookup_ReturnsModuleTemplateInfo_WhenNoMWRequired(t *testing.T) {
	maintenanceWindow := &maintenanceWindowStub{
		required: false,
	}
	expectedModuleTemplateInfo := templatelookup.ModuleTemplateInfo{
		DesiredChannel: "test",
		ModuleTemplate: &v1beta2.ModuleTemplate{
			Spec: v1beta2.ModuleTemplateSpec{
				Channel: "test",
			},
		},
	}
	decorated := &lookupStrategyStub{
		moduleTemplateInfo: expectedModuleTemplateInfo,
	}
	withMaintenanceWindowDecorator := moduletemplateinfolookup.NewWithMaintenanceWindowDecorator(maintenanceWindow,
		decorated)

	moduleTemplateInfo := withMaintenanceWindowDecorator.Lookup(t.Context(),
		nil,
		nil,
		nil)

	assert.True(t, maintenanceWindow.requiredCalled)
	assert.False(t, maintenanceWindow.activeCalled)
	assert.Equal(t, expectedModuleTemplateInfo, moduleTemplateInfo)
}

func Test_WithMWDecorator_Lookup_ReturnsError_WhenIsActiveReturnsError(t *testing.T) {
	err := errors.New("test error")
	maintenanceWindow := &maintenanceWindowStub{
		required: true,
		err:      err,
	}
	decorated := &lookupStrategyStub{
		moduleTemplateInfo: templatelookup.ModuleTemplateInfo{
			ModuleTemplate: &v1beta2.ModuleTemplate{
				Spec: v1beta2.ModuleTemplateSpec{
					Channel: "test",
				},
			},
		},
	}
	withMaintenanceWindowDecorator := moduletemplateinfolookup.NewWithMaintenanceWindowDecorator(maintenanceWindow,
		decorated)

	moduleTemplateInfo := withMaintenanceWindowDecorator.Lookup(t.Context(),
		nil,
		nil,
		nil)

	assert.True(t, maintenanceWindow.requiredCalled)
	assert.True(t, maintenanceWindow.activeCalled)
	require.ErrorIs(t, moduleTemplateInfo.Err, moduletemplateinfolookup.ErrFailedToDetermineIfMaintenanceWindowIsActive)
	require.ErrorIs(t, moduleTemplateInfo.Err, err)
	assert.Nil(t, moduleTemplateInfo.ModuleTemplate)
}

func Test_WithMWDecorator_Lookup_ReturnsError_WhenMWIsRequiredAndNotActive(t *testing.T) {
	maintenanceWindow := &maintenanceWindowStub{
		required: true,
		active:   false,
	}
	decorated := &lookupStrategyStub{
		moduleTemplateInfo: templatelookup.ModuleTemplateInfo{
			ModuleTemplate: &v1beta2.ModuleTemplate{
				Spec: v1beta2.ModuleTemplateSpec{
					Channel: "test",
				},
			},
		},
	}
	withMaintenanceWindowDecorator := moduletemplateinfolookup.NewWithMaintenanceWindowDecorator(maintenanceWindow,
		decorated)

	moduleTemplateInfo := withMaintenanceWindowDecorator.Lookup(t.Context(),
		nil,
		nil,
		nil)

	assert.True(t, maintenanceWindow.requiredCalled)
	assert.True(t, maintenanceWindow.activeCalled)
	require.ErrorIs(t, moduleTemplateInfo.Err, moduletemplateinfolookup.ErrWaitingForNextMaintenanceWindow)
	assert.Nil(t, moduleTemplateInfo.ModuleTemplate)
}

func Test_WithMWDecorator_Lookup_ReturnsModuleTemplateInfo_WhenMWIsRequiredAndActive(t *testing.T) {
	maintenanceWindow := &maintenanceWindowStub{
		required: true,
		active:   true,
	}
	expectedModuleTemplateInfo := templatelookup.ModuleTemplateInfo{
		DesiredChannel: "test",
		ModuleTemplate: &v1beta2.ModuleTemplate{
			Spec: v1beta2.ModuleTemplateSpec{
				Channel: "test",
			},
		},
	}
	decorated := &lookupStrategyStub{
		moduleTemplateInfo: expectedModuleTemplateInfo,
	}
	withMaintenanceWindowDecorator := moduletemplateinfolookup.NewWithMaintenanceWindowDecorator(maintenanceWindow,
		decorated)

	moduleTemplateInfo := withMaintenanceWindowDecorator.Lookup(t.Context(),
		nil,
		nil,
		nil)

	assert.True(t, maintenanceWindow.requiredCalled)
	assert.True(t, maintenanceWindow.activeCalled)
	assert.Equal(t, expectedModuleTemplateInfo, moduleTemplateInfo)
}

type lookupStrategyStub struct {
	responsible        bool
	called             bool
	moduleTemplateInfo templatelookup.ModuleTemplateInfo
}

func (s *lookupStrategyStub) IsResponsible(_ *templatelookup.ModuleInfo, _ *v1beta2.ModuleReleaseMeta) bool {
	s.called = true
	return s.responsible
}

func (s *lookupStrategyStub) Lookup(_ context.Context,
	_ *templatelookup.ModuleInfo,
	_ *v1beta2.Kyma,
	_ *v1beta2.ModuleReleaseMeta,
) templatelookup.ModuleTemplateInfo {
	return s.moduleTemplateInfo
}

type maintenanceWindowStub struct {
	requiredCalled bool
	required       bool
	activeCalled   bool
	active         bool
	err            error
}

func (s *maintenanceWindowStub) IsRequired(_ *v1beta2.ModuleTemplate, _ *v1beta2.Kyma) bool {
	s.requiredCalled = true
	return s.required
}

func (s *maintenanceWindowStub) IsActive(_ *v1beta2.Kyma) (bool, error) {
	s.activeCalled = true
	if s.err != nil {
		return false, s.err
	}
	return s.active, nil
}
