package moduletemplateinfolookup_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func Test_ByVersionStrategy_IsResponsible_ReturnsTrue(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithVersion("1.0.0").Enabled().Build()
	var kyma *v1beta2.Kyma = nil
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil
	byVersionStrategy := moduletemplateinfolookup.NewByVersionStrategy(nil)

	responsible := byVersionStrategy.IsResponsible(moduleInfo, kyma, moduleReleaseMeta)

	assert.True(t, responsible)
}

func Test_ByVersionStrategy_IsResponsible_ReturnsFalse_WhenModuleReleaseMetaIsNotNil(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithVersion("1.0.0").Enabled().Build()
	var kyma *v1beta2.Kyma = nil
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = builder.NewModuleReleaseMetaBuilder().Build()
	byVersionStrategy := moduletemplateinfolookup.NewByVersionStrategy(nil)

	responsible := byVersionStrategy.IsResponsible(moduleInfo, kyma, moduleReleaseMeta)

	assert.False(t, responsible)
}

func Test_ByVersionStrategy_IsResponsible_ReturnsFalse_WhenNotInstalledByVersion(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithVersion("").WithChannel("regular").Enabled().Build()
	var kyma *v1beta2.Kyma = nil
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil
	byVersionStrategy := moduletemplateinfolookup.NewByVersionStrategy(nil)

	responsible := byVersionStrategy.IsResponsible(moduleInfo, kyma, moduleReleaseMeta)

	assert.False(t, responsible)
}

func Test_ByVersion_Strategy_Lookup_ReturnsModuleTemplateInfo(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithName("test-module").WithVersion("1.0.0").Enabled().Build()
	var kyma *v1beta2.Kyma = nil
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithName("test-module-1.0.0").
		WithModuleName("test-module").
		WithVersion("1.0.0").
		WithChannel("none").
		Build()
	byVersionStrategy := moduletemplateinfolookup.NewByVersionStrategy(fakeClient(
		&v1beta2.ModuleTemplateList{
			Items: []v1beta2.ModuleTemplate{
				*moduleTemplate,
			},
		},
	))

	moduleTemplateInfo := byVersionStrategy.Lookup(nil, moduleInfo, kyma, moduleReleaseMeta)

	assert.NotNil(t, moduleTemplateInfo)
	assert.Equal(t, moduleTemplate.Name, moduleTemplateInfo.ModuleTemplate.Name)
	assert.Equal(t, moduleTemplate.Spec.ModuleName, moduleTemplateInfo.ModuleTemplate.Spec.ModuleName)
	assert.Equal(t, moduleTemplate.Spec.Version, moduleTemplateInfo.ModuleTemplate.Spec.Version)
	assert.Equal(t, moduleTemplate.Spec.Channel, moduleTemplateInfo.ModuleTemplate.Spec.Channel)
}

type moduleInfoBuilder struct {
	moduleInfo *templatelookup.ModuleInfo
}

func newModuleInfoBuilder() moduleInfoBuilder {
	return moduleInfoBuilder{
		moduleInfo: &templatelookup.ModuleInfo{
			Module: v1beta2.Module{},
		},
	}
}

func (b moduleInfoBuilder) WithName(name string) moduleInfoBuilder {
	b.moduleInfo.Module.Name = name
	return b
}

func (b moduleInfoBuilder) WithVersion(version string) moduleInfoBuilder {
	b.moduleInfo.Module.Version = version
	return b
}

func (b moduleInfoBuilder) WithChannel(channel string) moduleInfoBuilder {
	b.moduleInfo.Module.Channel = channel
	return b
}

func (b moduleInfoBuilder) Enabled() moduleInfoBuilder {
	b.moduleInfo.Enabled = true
	return b
}

func (b moduleInfoBuilder) Build() *templatelookup.ModuleInfo {
	return b.moduleInfo
}
