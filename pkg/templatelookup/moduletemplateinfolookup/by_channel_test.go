package moduletemplateinfolookup_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func Test_ByChannelStrategy_IsResponsible_ReturnsTrue(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithChannel("regular").Enabled().Build()
	var kyma *v1beta2.Kyma = nil
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil
	byChannelStrategy := moduletemplateinfolookup.NewByChannelStrategy(nil)

	responsible := byChannelStrategy.IsResponsible(moduleInfo, kyma, moduleReleaseMeta)

	assert.True(t, responsible)
}

func Test_ByChannelStrategy_IsResponsible_ReturnsFalse_WhenModuleReleaseMetaIsNotNil(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithChannel("regular").Enabled().Build()
	var kyma *v1beta2.Kyma = nil
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = builder.NewModuleReleaseMetaBuilder().Build()
	byChannelStrategy := moduletemplateinfolookup.NewByChannelStrategy(nil)

	responsible := byChannelStrategy.IsResponsible(moduleInfo, kyma, moduleReleaseMeta)

	assert.False(t, responsible)
}

func Test_ByChannelStrategy_IsResponsible_ReturnsFalse_WhenInstalledByVersion(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithVersion("1.0.0").Enabled().Build()
	var kyma *v1beta2.Kyma = nil
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil
	byChannelStrategy := moduletemplateinfolookup.NewByChannelStrategy(nil)

	responsible := byChannelStrategy.IsResponsible(moduleInfo, kyma, moduleReleaseMeta)

	assert.False(t, responsible)
}

func Test_ByChannelStrategy_Lookup_ReturnsModuleTemplateInfo(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithName("test-module").WithChannel("regular").Enabled().Build()
	var kyma *v1beta2.Kyma = builder.NewKymaBuilder().Build()
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

	moduleTemplateInfo := byChannelStrategy.Lookup(nil, moduleInfo, kyma, moduleReleaseMeta)

	assert.NotNil(t, moduleTemplateInfo)
	assert.Equal(t, moduleTemplate.Name, moduleTemplateInfo.ModuleTemplate.Name)
	assert.Equal(t, moduleTemplate.Spec.ModuleName, moduleTemplateInfo.ModuleTemplate.Spec.ModuleName)
	assert.Equal(t, moduleTemplate.Spec.Version, moduleTemplateInfo.ModuleTemplate.Spec.Version)
	assert.Equal(t, moduleTemplate.Spec.Channel, moduleTemplateInfo.ModuleTemplate.Spec.Channel)
}
