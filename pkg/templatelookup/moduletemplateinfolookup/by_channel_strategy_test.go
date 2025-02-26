package moduletemplateinfolookup_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func Test_ByChannelStrategy_IsResponsible_ReturnsTrue(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithChannel("regular").Enabled().Build()
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil
	byChannelStrategy := moduletemplateinfolookup.NewByChannelStrategy(nil)

	responsible := byChannelStrategy.IsResponsible(moduleInfo, moduleReleaseMeta)

	assert.True(t, responsible)
}

func Test_ByChannelStrategy_IsResponsible_ReturnsFalse_WhenModuleReleaseMetaIsNotNil(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithChannel("regular").Enabled().Build()
	moduleReleaseMeta := builder.NewModuleReleaseMetaBuilder().Build()
	byChannelStrategy := moduletemplateinfolookup.NewByChannelStrategy(nil)

	responsible := byChannelStrategy.IsResponsible(moduleInfo, moduleReleaseMeta)

	assert.False(t, responsible)
}

func Test_ByChannelStrategy_IsResponsible_ReturnsFalse_WhenInstalledByVersion(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithVersion("1.0.0").Enabled().Build()
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil
	byChannelStrategy := moduletemplateinfolookup.NewByChannelStrategy(nil)

	responsible := byChannelStrategy.IsResponsible(moduleInfo, moduleReleaseMeta)

	assert.False(t, responsible)
}

func Test_ByChannelStrategy_Lookup_ReturnsModuleTemplateInfo(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithName("test-module").WithChannel("regular").Enabled().Build()
	kyma := builder.NewKymaBuilder().Build()
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithName("test-module-regular").
		WithModuleName("test-module").
		WithVersion("").
		WithChannel("regular").
		Build()
	byChannelStrategy := moduletemplateinfolookup.NewByChannelStrategy(fakeClient(
		&v1beta2.ModuleTemplateList{
			Items: []v1beta2.ModuleTemplate{
				*moduleTemplate,
			},
		},
	))

	moduleTemplateInfo := byChannelStrategy.Lookup(context.Background(), moduleInfo, kyma, moduleReleaseMeta)

	assert.NotNil(t, moduleTemplateInfo)
	assert.Equal(t, moduleTemplate.Name, moduleTemplateInfo.ModuleTemplate.Name)
	assert.Equal(t, moduleTemplate.Spec.ModuleName, moduleTemplateInfo.ModuleTemplate.Spec.ModuleName)
	assert.Equal(t, moduleTemplate.Spec.Version, moduleTemplateInfo.ModuleTemplate.Spec.Version)
	assert.Equal(t, moduleTemplate.Spec.Channel, moduleTemplateInfo.ModuleTemplate.Spec.Channel)
}

func Test_ByChannelStrategy_Lookup_ReturnsModuleTemplateInfo_UsingGlobalChannel(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithName("test-module").Enabled().Build()
	kyma := builder.NewKymaBuilder().WithChannel("fast").Build()
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithName("test-module-fast").
		WithModuleName("test-module").
		WithVersion("").
		WithChannel("fast").
		Build()
	byChannelStrategy := moduletemplateinfolookup.NewByChannelStrategy(fakeClient(
		&v1beta2.ModuleTemplateList{
			Items: []v1beta2.ModuleTemplate{
				*moduleTemplate,
			},
		},
	))

	moduleTemplateInfo := byChannelStrategy.Lookup(context.Background(), moduleInfo, kyma, moduleReleaseMeta)

	assert.NotNil(t, moduleTemplateInfo)
	assert.Equal(t, moduleTemplate.Name, moduleTemplateInfo.ModuleTemplate.Name)
	assert.Equal(t, moduleTemplate.Spec.ModuleName, moduleTemplateInfo.ModuleTemplate.Spec.ModuleName)
	assert.Equal(t, moduleTemplate.Spec.Version, moduleTemplateInfo.ModuleTemplate.Spec.Version)
	assert.Equal(t, moduleTemplate.Spec.Channel, moduleTemplateInfo.ModuleTemplate.Spec.Channel)
}

func Test_ByChannelStrategy_Lookup_ReturnsModuleTemplateInfo_UsingDefaultChannel(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithName("test-module").Enabled().Build()
	kyma := builder.NewKymaBuilder().Build()
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithName("test-module-regular").
		WithModuleName("test-module").
		WithVersion("").
		WithChannel("regular").
		Build()
	byChannelStrategy := moduletemplateinfolookup.NewByChannelStrategy(fakeClient(
		&v1beta2.ModuleTemplateList{
			Items: []v1beta2.ModuleTemplate{
				*moduleTemplate,
			},
		},
	))

	moduleTemplateInfo := byChannelStrategy.Lookup(context.Background(), moduleInfo, kyma, moduleReleaseMeta)

	assert.NotNil(t, moduleTemplateInfo)
	assert.Equal(t, moduleTemplate.Name, moduleTemplateInfo.ModuleTemplate.Name)
	assert.Equal(t, moduleTemplate.Spec.ModuleName, moduleTemplateInfo.ModuleTemplate.Spec.ModuleName)
	assert.Equal(t, moduleTemplate.Spec.Version, moduleTemplateInfo.ModuleTemplate.Spec.Version)
	assert.Equal(t, moduleTemplate.Spec.Channel, moduleTemplateInfo.ModuleTemplate.Spec.Channel)
}

func Test_ByChannelStrategy_Lookup_WhenNoModuleTemplateFound(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithName("test-module").WithChannel("regular").Enabled().Build()
	kyma := builder.NewKymaBuilder().Build()
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil
	byChannelStrategy := moduletemplateinfolookup.NewByChannelStrategy(fakeClient(&v1beta2.ModuleTemplateList{
		Items: []v1beta2.ModuleTemplate{},
	}))

	moduleTemplateInfo := byChannelStrategy.Lookup(context.Background(), moduleInfo, kyma, moduleReleaseMeta)

	assert.NotNil(t, moduleTemplateInfo)
	assert.Nil(t, moduleTemplateInfo.ModuleTemplate)
	assert.ErrorContains(t, moduleTemplateInfo.Err,
		"no templates were found: for module test-module in channel regular")
}

func Test_ByChannelStrategy_Lookup_WhenMoreThanOneModuleTemplateFound(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithName("test-module").WithChannel("regular").Enabled().Build()
	kyma := builder.NewKymaBuilder().Build()
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil
	firstModuleTemplate := builder.NewModuleTemplateBuilder().
		WithName("test-module-regular").
		WithModuleName("test-module").
		WithVersion("").
		WithChannel("regular").
		Build()
	secondModuleTemplate := builder.NewModuleTemplateBuilder().
		WithName("test-module-regular-2").
		WithModuleName("test-module").
		WithVersion("").
		WithChannel("regular").
		Build()
	byChannelStrategy := moduletemplateinfolookup.NewByChannelStrategy(fakeClient(
		&v1beta2.ModuleTemplateList{
			Items: []v1beta2.ModuleTemplate{
				*firstModuleTemplate,
				*secondModuleTemplate,
			},
		},
	))

	moduleTemplateInfo := byChannelStrategy.Lookup(context.Background(), moduleInfo, kyma, moduleReleaseMeta)

	assert.NotNil(t, moduleTemplateInfo)
	assert.Nil(t, moduleTemplateInfo.ModuleTemplate)
	assert.ErrorContains(t, moduleTemplateInfo.Err,
		"no unique template could be identified: more than one module template found for module: test-module, candidates: [test-module-regular test-module-regular-2]")
}

func Test_ByChannelStrategy_Lookup_WhenModuleTemplateHasNoChannel(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithName("test-module").WithChannel("regular").Enabled().Build()
	kyma := builder.NewKymaBuilder().Build()
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithName("test-module-regular").
		WithModuleName("test-module").
		WithVersion("").
		Build()
	byChannelStrategy := moduletemplateinfolookup.NewByChannelStrategy(fakeClient(
		&v1beta2.ModuleTemplateList{
			Items: []v1beta2.ModuleTemplate{
				*moduleTemplate,
			},
		},
	))

	moduleTemplateInfo := byChannelStrategy.Lookup(context.Background(), moduleInfo, kyma, moduleReleaseMeta)

	assert.NotNil(t, moduleTemplateInfo)
	assert.Nil(t, moduleTemplateInfo.ModuleTemplate)
	assert.ErrorContains(t, moduleTemplateInfo.Err,
		"no templates were found: for module test-module in channel regular")
}

func Test_ByChannelStrategy_Lookup_WhenModuleTemplateIsMandatory(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithName("test-module").WithChannel("regular").Enabled().Build()
	kyma := builder.NewKymaBuilder().Build()
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithName("test-module-regular").
		WithModuleName("test-module").
		WithChannel("regular").
		WithMandatory(true).
		WithVersion("").
		Build()
	byChannelStrategy := moduletemplateinfolookup.NewByChannelStrategy(fakeClient(
		&v1beta2.ModuleTemplateList{
			Items: []v1beta2.ModuleTemplate{
				*moduleTemplate,
			},
		},
	))

	moduleTemplateInfo := byChannelStrategy.Lookup(context.Background(), moduleInfo, kyma, moduleReleaseMeta)

	assert.NotNil(t, moduleTemplateInfo)
	assert.Nil(t, moduleTemplateInfo.ModuleTemplate)
	assert.ErrorContains(t, moduleTemplateInfo.Err,
		"template marked as mandatory: for module test-module in channel regular")
}
