package moduletemplateinfolookup_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	machineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/lifecycle-manager/api"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func Test_ByModuleReleaseMetaStrategy_IsResponsible_ReturnsTrue(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithChannel("regular").Enabled().Build()
	var kyma *v1beta2.Kyma = nil
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = builder.NewModuleReleaseMetaBuilder().Build()
	byMRMStrategy := moduletemplateinfolookup.NewByModuleReleaseMetaStrategy(nil)

	responsible := byMRMStrategy.IsResponsible(moduleInfo, kyma, moduleReleaseMeta)

	assert.True(t, responsible)
}

func Test_ByModuleReleaseMetaStrategy_IsResponsible_ReturnsFalse_WhenModuleReleaseMetaIsNotNil(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithVersion("regular").Enabled().Build()
	var kyma *v1beta2.Kyma = nil
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = nil
	byMRMStrategy := moduletemplateinfolookup.NewByModuleReleaseMetaStrategy(nil)

	responsible := byMRMStrategy.IsResponsible(moduleInfo, kyma, moduleReleaseMeta)

	assert.False(t, responsible)
}

func Test_ByModuleReleaseMeta_Strategy_Lookup_ReturnsModuleTemplateInfo(t *testing.T) {
	moduleInfo := newModuleInfoBuilder().WithName("test-module").WithChannel("regular").Enabled().Build()
	var kyma *v1beta2.Kyma = builder.NewKymaBuilder().Build()
	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta = builder.NewModuleReleaseMetaBuilder().
		WithModuleName("test-module").
		WithName("test-module").
		WithModuleChannelAndVersions([]v1beta2.ChannelVersionAssignment{
			{
				Channel: "regular",
				Version: "1.0.0",
			},
		}).
		Build()
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithName("test-module-1.0.0").
		WithModuleName("test-module").
		WithVersion("1.0.0").
		WithChannel("none").
		Build()
	byMRMStrategy := moduletemplateinfolookup.NewByModuleReleaseMetaStrategy(fakeClient(
		&v1beta2.ModuleTemplateList{
			Items: []v1beta2.ModuleTemplate{
				*moduleTemplate,
			},
		},
	))

	moduleTemplateInfo := byMRMStrategy.Lookup(nil, moduleInfo, kyma, moduleReleaseMeta)

	assert.NotNil(t, moduleTemplateInfo)
	assert.Equal(t, moduleTemplate.Name, moduleTemplateInfo.ModuleTemplate.Name)
	assert.Equal(t, moduleTemplate.Spec.ModuleName, moduleTemplateInfo.ModuleTemplate.Spec.ModuleName)
	assert.Equal(t, moduleTemplate.Spec.Version, moduleTemplateInfo.ModuleTemplate.Spec.Version)
	assert.Equal(t, moduleTemplate.Spec.Channel, moduleTemplateInfo.ModuleTemplate.Spec.Channel)
}

func fakeClient(mts *v1beta2.ModuleTemplateList) client.Client {
	scheme := machineryruntime.NewScheme()
	machineryutilruntime.Must(api.AddToScheme(scheme))

	return fake.NewClientBuilder().WithScheme(scheme).WithLists(mts).Build()
}

// func fakeClient(mts *v1beta2.ModuleTemplateList) client.Client {
// 	scheme := machineryruntime.NewScheme()
// 	machineryutilruntime.Must(api.AddToScheme(scheme))

// 	return fake.NewClientBuilder().WithScheme(scheme).WithLists(mts).Build()
// }

// type moduleInfoBuilder struct {
// 	moduleInfo *templatelookup.ModuleInfo
// }

// func newModuleInfoBuilder() moduleInfoBuilder {
// 	return moduleInfoBuilder{
// 		moduleInfo: &templatelookup.ModuleInfo{
// 			Module: v1beta2.Module{},
// 		},
// 	}
// }

// func (b moduleInfoBuilder) WithName(name string) moduleInfoBuilder {
// 	b.moduleInfo.Module.Name = name
// 	return b
// }

// func (b moduleInfoBuilder) WithVersion(version string) moduleInfoBuilder {
// 	b.moduleInfo.Module.Version = version
// 	return b
// }

// func (b moduleInfoBuilder) WithChannel(channel string) moduleInfoBuilder {
// 	b.moduleInfo.Module.Channel = channel
// 	return b
// }

// func (b moduleInfoBuilder) Enabled() moduleInfoBuilder {
// 	b.moduleInfo.Enabled = true
// 	return b
// }

// func (b moduleInfoBuilder) Build() *templatelookup.ModuleInfo {
// 	return b.moduleInfo
// }
