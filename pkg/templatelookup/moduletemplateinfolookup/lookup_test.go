package moduletemplateinfolookup_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	machineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/lifecycle-manager/api"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func Test_ModuleTemplateInfoLookup_ReturnsModuleTemplateInfo(t *testing.T) {
	moduleInfo := &templatelookup.ModuleInfo{
		Module: v1beta2.Module{
			Name:    "test-module",
			Channel: "regular",
		},
		Enabled: true,
	}
	kyma := builder.NewKymaBuilder().Build()
	moduleReleaseMeta := builder.NewModuleReleaseMetaBuilder().
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
		Build()
	lookup := moduletemplateinfolookup.New(fakeClient(
		&v1beta2.ModuleTemplateList{
			Items: []v1beta2.ModuleTemplate{
				*moduleTemplate,
			},
		},
	))

	moduleTemplateInfo := lookup.Lookup(context.Background(), moduleInfo, kyma, moduleReleaseMeta)

	require.NotNil(t, moduleTemplateInfo)
	require.NoError(t, moduleTemplateInfo.Err)
	require.Equal(t, moduleTemplate.Name, moduleTemplateInfo.ModuleTemplate.Name)
	require.Equal(t, moduleTemplate.Spec.ModuleName, moduleTemplateInfo.ModuleTemplate.Spec.ModuleName)
	require.Equal(t, moduleTemplate.Spec.Version, moduleTemplateInfo.ModuleTemplate.Spec.Version)
}

func Test_ModuleTemplateInfoLookup_WhenMandatoryModuleActivated_ReturnsModuleTemplateInfo(t *testing.T) {
	moduleInfo := &templatelookup.ModuleInfo{
		Module: v1beta2.Module{
			Name:    "test-module",
			Channel: "regular",
		},
		Enabled: true,
	}
	kyma := builder.NewKymaBuilder().Build()
	moduleReleaseMeta := builder.NewModuleReleaseMetaBuilder().
		WithModuleName("test-module").
		WithName("test-module").
		WithMandatory("1.0.0").
		Build()
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithName("test-module-1.0.0").
		WithModuleName("test-module").
		WithMandatory(true).
		WithVersion("1.0.0").
		Build()
	lookup := moduletemplateinfolookup.New(fakeClient(
		&v1beta2.ModuleTemplateList{
			Items: []v1beta2.ModuleTemplate{
				*moduleTemplate,
			},
		},
	))

	moduleTemplateInfo := lookup.Lookup(context.Background(), moduleInfo, kyma, moduleReleaseMeta)

	require.NotNil(t, moduleTemplateInfo)
	require.NoError(t, moduleTemplateInfo.Err)
	require.Equal(t, moduleTemplate.Name, moduleTemplateInfo.ModuleTemplate.Name)
	require.Equal(t, moduleTemplate.Spec.ModuleName, moduleTemplateInfo.ModuleTemplate.Spec.ModuleName)
	require.Equal(t, moduleTemplate.Spec.Version, moduleTemplateInfo.ModuleTemplate.Spec.Version)
}

func fakeClient(mts *v1beta2.ModuleTemplateList) client.Client {
	scheme := machineryruntime.NewScheme()
	machineryutilruntime.Must(api.AddToScheme(scheme))

	return fake.NewClientBuilder().WithScheme(scheme).WithLists(mts).Build()
}
