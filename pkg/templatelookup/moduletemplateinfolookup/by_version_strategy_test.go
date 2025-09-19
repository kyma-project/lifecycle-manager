package moduletemplateinfolookup_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func Test_ByVersionStrategy_IsResponsible_ReturnsTrue(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithVersion("1.0.0").Enabled().Build()
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil
	byVersionStrategy := moduletemplateinfolookup.NewByVersionStrategy(nil)

	responsible := byVersionStrategy.IsResponsible(moduleInfo, moduleReleaseMeta)

	require.True(t, responsible)
}

func Test_ByVersionStrategy_IsResponsible_ReturnsFalse_WhenModuleReleaseMetaIsNotNil(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithVersion("1.0.0").Enabled().Build()
	moduleReleaseMeta := builder.NewModuleReleaseMetaBuilder().Build()
	byVersionStrategy := moduletemplateinfolookup.NewByVersionStrategy(nil)

	responsible := byVersionStrategy.IsResponsible(moduleInfo, moduleReleaseMeta)

	require.False(t, responsible)
}

func Test_ByVersionStrategy_IsResponsible_ReturnsFalse_WhenNotInstalledByVersion(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithVersion("").WithChannel("regular").Enabled().Build()
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil
	byVersionStrategy := moduletemplateinfolookup.NewByVersionStrategy(nil)

	responsible := byVersionStrategy.IsResponsible(moduleInfo, moduleReleaseMeta)

	require.False(t, responsible)
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

	moduleTemplateInfo := byVersionStrategy.Lookup(t.Context(), moduleInfo, kyma, moduleReleaseMeta)

	require.NotNil(t, moduleTemplateInfo)
	require.Equal(t, moduleTemplate.Name, moduleTemplateInfo.Name)
	require.Equal(t, moduleTemplate.Spec.ModuleName, moduleTemplateInfo.Spec.ModuleName)
	require.Equal(t, moduleTemplate.Spec.Version, moduleTemplateInfo.Spec.Version)
	require.Equal(t, moduleTemplate.Spec.Channel, moduleTemplateInfo.Spec.Channel)
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

	moduleTemplateInfo := byVersionStrategy.Lookup(t.Context(), moduleInfo, kyma, moduleReleaseMeta)

	require.NotNil(t, moduleTemplateInfo)
	require.Nil(t, moduleTemplateInfo.ModuleTemplate)
	require.ErrorContains(
		t,
		moduleTemplateInfo.Err,
		"no unique template could be identified: more than one module template found for module: test-module, "+
			"candidates: [test-module-1.0.0 test-module-1.0.0-duplicate]",
	)
}

func Test_ByVersion_Strategy_Lookup_WhenFailedToListModuleTemplates(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithName("test-module").WithVersion("1.0.0").Enabled().Build()
	var kyma *v1beta2.Kyma = nil
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil

	byVersionStrategy := moduletemplateinfolookup.NewByVersionStrategy(&failedClientStub{})

	moduleTemplateInfo := byVersionStrategy.Lookup(t.Context(), moduleInfo, kyma, moduleReleaseMeta)

	require.NotNil(t, moduleTemplateInfo)
	require.Nil(t, moduleTemplateInfo.ModuleTemplate)
	require.ErrorContains(t, moduleTemplateInfo.Err,
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

	moduleTemplateInfo := byVersionStrategy.Lookup(t.Context(), moduleInfo, kyma, moduleReleaseMeta)

	require.NotNil(t, moduleTemplateInfo)
	require.Nil(t, moduleTemplateInfo.ModuleTemplate)
	require.ErrorContains(t, moduleTemplateInfo.Err,
		"no templates were found: for module test-module in version 1.0.0")
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
	b.moduleInfo.Name = name
	return b
}

func (b moduleInfoBuilder) WithVersion(version string) moduleInfoBuilder {
	b.moduleInfo.Version = version
	return b
}

func (b moduleInfoBuilder) WithChannel(channel string) moduleInfoBuilder {
	b.moduleInfo.Channel = channel
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

func (c *failedClientStub) List(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
	return errors.New("failed to list module templates")
}
