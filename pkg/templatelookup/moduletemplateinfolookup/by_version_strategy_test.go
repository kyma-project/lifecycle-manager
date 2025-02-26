package moduletemplateinfolookup_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"errors"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Test_ByVersionStrategy_IsResponsible_ReturnsTrue(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithVersion("1.0.0").Enabled().Build()
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil
	byVersionStrategy := moduletemplateinfolookup.NewByVersionStrategy(nil)

	responsible := byVersionStrategy.IsResponsible(moduleInfo, moduleReleaseMeta)

	assert.True(t, responsible)
}

func Test_ByVersionStrategy_IsResponsible_ReturnsFalse_WhenModuleReleaseMetaIsNotNil(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithVersion("1.0.0").Enabled().Build()
	moduleReleaseMeta := builder.NewModuleReleaseMetaBuilder().Build()
	byVersionStrategy := moduletemplateinfolookup.NewByVersionStrategy(nil)

	responsible := byVersionStrategy.IsResponsible(moduleInfo, moduleReleaseMeta)

	assert.False(t, responsible)
}

func Test_ByVersionStrategy_IsResponsible_ReturnsFalse_WhenNotInstalledByVersion(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithVersion("").WithChannel("regular").Enabled().Build()
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil
	byVersionStrategy := moduletemplateinfolookup.NewByVersionStrategy(nil)

	responsible := byVersionStrategy.IsResponsible(moduleInfo, moduleReleaseMeta)

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

	moduleTemplateInfo := byVersionStrategy.Lookup(context.Background(), moduleInfo, kyma, moduleReleaseMeta)

	assert.NotNil(t, moduleTemplateInfo)
	assert.Equal(t, moduleTemplate.Name, moduleTemplateInfo.ModuleTemplate.Name)
	assert.Equal(t, moduleTemplate.Spec.ModuleName, moduleTemplateInfo.ModuleTemplate.Spec.ModuleName)
	assert.Equal(t, moduleTemplate.Spec.Version, moduleTemplateInfo.ModuleTemplate.Spec.Version)
	assert.Equal(t, moduleTemplate.Spec.Channel, moduleTemplateInfo.ModuleTemplate.Spec.Channel)
}

func Test_ByVersion_Strategy_Lookup_WhenMoreThanOneModuleTemplateFound(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithName("test-module").WithVersion("1.0.0").Enabled().Build()
	var kyma *v1beta2.Kyma = nil
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil
	firstModuleTemplate := builder.NewModuleTemplateBuilder().
		WithName("test-module-1.0.0").
		WithModuleName("test-module").
		WithVersion("1.0.0").
		WithChannel("none").
		Build()
	secondModuleTemplate := builder.NewModuleTemplateBuilder().
		WithName("test-module-1.0.0-duplicate").
		WithModuleName("test-module").
		WithVersion("1.0.0").
		WithChannel("none").
		Build()

	byVersionStrategy := moduletemplateinfolookup.NewByVersionStrategy(fakeClient(
		&v1beta2.ModuleTemplateList{
			Items: []v1beta2.ModuleTemplate{
				*firstModuleTemplate,
				*secondModuleTemplate,
			},
		},
	))

	moduleTemplateInfo := byVersionStrategy.Lookup(context.Background(), moduleInfo, kyma, moduleReleaseMeta)

	assert.NotNil(t, moduleTemplateInfo)
	assert.Nil(t, moduleTemplateInfo.ModuleTemplate)
	assert.ErrorContains(t, moduleTemplateInfo.Err,
		"no unique template could be identified: more than one module template found for module: test-module, candidates: [test-module-1.0.0 test-module-1.0.0-duplicate]")
}

func Test_ByVersion_Strategy_Lookup_WhenFailedToListModuleTemplates(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithName("test-module").WithVersion("1.0.0").Enabled().Build()
	var kyma *v1beta2.Kyma = nil
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil

	byVersionStrategy := moduletemplateinfolookup.NewByVersionStrategy(&failedClientStub{})

	moduleTemplateInfo := byVersionStrategy.Lookup(context.Background(), moduleInfo, kyma, moduleReleaseMeta)

	assert.NotNil(t, moduleTemplateInfo)
	assert.Nil(t, moduleTemplateInfo.ModuleTemplate)
	assert.ErrorContains(t, moduleTemplateInfo.Err,
		"failed to list module templates on lookup")
}

func Test_ByVersion_Strategy_Lookup_WhenNoModuleTemplateFound(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithName("test-module").WithVersion("1.0.0").Enabled().Build()
	var kyma *v1beta2.Kyma = nil
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil

	byVersionStrategy := moduletemplateinfolookup.NewByVersionStrategy(fakeClient(
		&v1beta2.ModuleTemplateList{
			Items: []v1beta2.ModuleTemplate{},
		},
	))

	moduleTemplateInfo := byVersionStrategy.Lookup(context.Background(), moduleInfo, kyma, moduleReleaseMeta)

	assert.NotNil(t, moduleTemplateInfo)
	assert.Nil(t, moduleTemplateInfo.ModuleTemplate)
	assert.ErrorContains(t, moduleTemplateInfo.Err,
		"no templates were found: for module test-module in version 1.0.0")
}

func Test_ByVersion_Strategy_Lookup_WhenModuleTemplateIsMandatory(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithName("test-module").WithVersion("1.0.0").Enabled().Build()
	var kyma *v1beta2.Kyma = nil
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithName("test-module-1.0.0").
		WithModuleName("test-module").
		WithVersion("1.0.0").
		WithChannel("none").
		WithMandatory(true).
		Build()

	byVersionStrategy := moduletemplateinfolookup.NewByVersionStrategy(fakeClient(
		&v1beta2.ModuleTemplateList{
			Items: []v1beta2.ModuleTemplate{
				*moduleTemplate,
			},
		},
	))

	moduleTemplateInfo := byVersionStrategy.Lookup(context.Background(), moduleInfo, kyma, moduleReleaseMeta)

	assert.NotNil(t, moduleTemplateInfo)
	assert.Nil(t, moduleTemplateInfo.ModuleTemplate)
	assert.ErrorContains(t, moduleTemplateInfo.Err,
		"template marked as mandatory: for module test-module in version 1.0.0")
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

type failedClientStub struct {
	client.Client
}

func (c *failedClientStub) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return errors.New("failed to list module templates")
}
