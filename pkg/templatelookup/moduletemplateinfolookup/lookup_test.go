package moduletemplateinfolookup_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func TestLookup_WhenModuleReleaseMetaIsNil_ReturnsError(t *testing.T) {
	lookup := moduletemplateinfolookup.NewLookup(nil)

	result := lookup.Lookup(context.Background(), &templatelookup.ModuleInfo{}, &v1beta2.Kyma{}, nil)

	require.Error(t, result.Err)
	require.ErrorIs(t, moduletemplateinfolookup.ErrModuleReleaseMetaRequired, result.Err)
	assert.Nil(t, result.ModuleTemplate)
}

func TestLookup_WhenGetMandatoryVersionFails_ReturnsError(t *testing.T) {
	moduleReleaseMeta := builder.NewModuleReleaseMetaBuilder().
		WithModuleName("test-module").
		WithMandatory("invalid-version").
		Build()

	lookup := moduletemplateinfolookup.NewLookup(nil)

	result := lookup.Lookup(context.Background(),
		&templatelookup.ModuleInfo{
			Module: v1beta2.Module{Name: "test-module"},
		},
		&v1beta2.Kyma{},
		moduleReleaseMeta)

	require.Error(t, result.Err)
	assert.Nil(t, result.ModuleTemplate)
}

func TestLookup_WhenGetChannelVersionFails_ReturnsError(t *testing.T) {
	moduleReleaseMeta := builder.NewModuleReleaseMetaBuilder().
		WithModuleName("test-module").
		WithOcmComponentName("kyma-project.io/test-module").
		Build() // No channels defined

	lookup := moduletemplateinfolookup.NewLookup(nil)

	result := lookup.Lookup(context.Background(),
		&templatelookup.ModuleInfo{
			Module: v1beta2.Module{Name: "test-module"},
		},
		&v1beta2.Kyma{
			Spec: v1beta2.KymaSpec{
				Channel: "regular",
			},
		},
		moduleReleaseMeta)

	require.Error(t, result.Err)
	assert.Nil(t, result.ModuleTemplate)
}

func TestLookup_WhenOcmComponentNameIsInvalid_ReturnsError(t *testing.T) {
	moduleReleaseMeta := builder.NewModuleReleaseMetaBuilder().
		WithModuleName("test-module").
		WithOcmComponentName("").
		WithSingleModuleChannelAndVersions("regular", "1.0.0").
		Build()

	lookup := moduletemplateinfolookup.NewLookup(nil)

	result := lookup.Lookup(context.Background(),
		&templatelookup.ModuleInfo{
			Module: v1beta2.Module{Name: "test-module"},
		},
		&v1beta2.Kyma{
			Spec: v1beta2.KymaSpec{
				Channel: "regular",
			},
		},
		moduleReleaseMeta)

	require.Error(t, result.Err)
	assert.Nil(t, result.ModuleTemplate)
}

func TestLookup_WhenModuleTemplateNotFound_ReturnsError(t *testing.T) {
	scheme := machineryruntime.NewScheme()
	require.NoError(t, v1beta2.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	moduleReleaseMeta := builder.NewModuleReleaseMetaBuilder().
		WithModuleName("test-module").
		WithOcmComponentName("kyma-project.io/test-module").
		WithSingleModuleChannelAndVersions("regular", "1.0.0").
		Build()

	lookup := moduletemplateinfolookup.NewLookup(fakeClient)

	result := lookup.Lookup(context.Background(),
		&templatelookup.ModuleInfo{
			Module: v1beta2.Module{Name: "test-module"},
		},
		&v1beta2.Kyma{
			ObjectMeta: apimetav1.ObjectMeta{
				Namespace: "kyma-system",
			},
			Spec: v1beta2.KymaSpec{
				Channel: "regular",
			},
		},
		moduleReleaseMeta)

	require.Error(t, result.Err)
	require.ErrorContains(t, result.Err, "failed to get module template")
	assert.Nil(t, result.ModuleTemplate)
}

func TestLookup_WithMandatoryModule_Success(t *testing.T) {
	scheme := machineryruntime.NewScheme()
	require.NoError(t, v1beta2.AddToScheme(scheme))

	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithName(v1beta2.CreateModuleTemplateName("test-module", "1.0.0")).
		WithModuleName("test-module").
		WithVersion("1.0.0").
		WithNamespace("kyma-system").
		Build()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(moduleTemplate).
		Build()

	moduleReleaseMeta := builder.NewModuleReleaseMetaBuilder().
		WithModuleName("test-module").
		WithOcmComponentName("kyma-project.io/test-module").
		WithMandatory("1.0.0").
		Build()

	lookup := moduletemplateinfolookup.NewLookup(fakeClient)

	result := lookup.Lookup(context.Background(),
		&templatelookup.ModuleInfo{
			Module: v1beta2.Module{Name: "test-module"},
		},
		&v1beta2.Kyma{
			ObjectMeta: apimetav1.ObjectMeta{
				Namespace: "kyma-system",
			},
			Spec: v1beta2.KymaSpec{
				Channel: "regular",
			},
		},
		moduleReleaseMeta)

	require.NoError(t, result.Err)
	assert.NotNil(t, result.ModuleTemplate)
	assert.Equal(t, "test-module", result.Spec.ModuleName)
	assert.Equal(t, "1.0.0", result.Spec.Version)
	assert.Equal(t, "regular", result.DesiredChannel)
	assert.NotNil(t, result.ComponentId)
}

func TestLookup_WithChannelBasedModule_Success(t *testing.T) {
	scheme := machineryruntime.NewScheme()
	require.NoError(t, v1beta2.AddToScheme(scheme))

	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithName(v1beta2.CreateModuleTemplateName("test-module", "2.0.0")).
		WithModuleName("test-module").
		WithVersion("2.0.0").
		WithNamespace("kyma-system").
		Build()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(moduleTemplate).
		Build()

	moduleReleaseMeta := builder.NewModuleReleaseMetaBuilder().
		WithModuleName("test-module").
		WithOcmComponentName("kyma-project.io/test-module").
		WithSingleModuleChannelAndVersions("regular", "2.0.0").
		Build()

	lookup := moduletemplateinfolookup.NewLookup(fakeClient)

	result := lookup.Lookup(context.Background(),
		&templatelookup.ModuleInfo{
			Module: v1beta2.Module{Name: "test-module"},
		},
		&v1beta2.Kyma{
			ObjectMeta: apimetav1.ObjectMeta{
				Namespace: "kyma-system",
			},
			Spec: v1beta2.KymaSpec{
				Channel: "regular",
			},
		},
		moduleReleaseMeta)

	require.NoError(t, result.Err)
	assert.NotNil(t, result.ModuleTemplate)
	assert.Equal(t, "test-module", result.Spec.ModuleName)
	assert.Equal(t, "2.0.0", result.Spec.Version)
	assert.Equal(t, "regular", result.DesiredChannel)
	assert.NotNil(t, result.ComponentId)
}

func TestLookup_UsesModuleChannelOverKymaChannel(t *testing.T) {
	scheme := machineryruntime.NewScheme()
	require.NoError(t, v1beta2.AddToScheme(scheme))

	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithName(v1beta2.CreateModuleTemplateName("test-module", "3.0.0")).
		WithModuleName("test-module").
		WithVersion("3.0.0").
		WithNamespace("kyma-system").
		Build()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(moduleTemplate).
		Build()

	moduleReleaseMeta := builder.NewModuleReleaseMetaBuilder().
		WithModuleName("test-module").
		WithOcmComponentName("kyma-project.io/test-module").
		WithModuleChannelAndVersions([]v1beta2.ChannelVersionAssignment{
			{Channel: "fast", Version: "3.0.0"},
			{Channel: "regular", Version: "2.0.0"},
		}).
		Build()

	lookup := moduletemplateinfolookup.NewLookup(fakeClient)

	result := lookup.Lookup(context.Background(),
		&templatelookup.ModuleInfo{
			Module: v1beta2.Module{
				Name:    "test-module",
				Channel: "fast",
			},
		},
		&v1beta2.Kyma{
			ObjectMeta: apimetav1.ObjectMeta{
				Namespace: "kyma-system",
			},
			Spec: v1beta2.KymaSpec{
				Channel: "regular",
			},
		},
		moduleReleaseMeta)

	require.NoError(t, result.Err)
	assert.NotNil(t, result.ModuleTemplate)
	assert.Equal(t, "test-module", result.Spec.ModuleName)
	assert.Equal(t, "3.0.0", result.Spec.Version)
	assert.Equal(t, "fast", result.DesiredChannel) // Should use module channel, not kyma channel
	assert.NotNil(t, result.ComponentId)
}

func TestLookup_FallsBackToKymaChannelWhenModuleChannelEmpty(t *testing.T) {
	scheme := machineryruntime.NewScheme()
	require.NoError(t, v1beta2.AddToScheme(scheme))

	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithName(v1beta2.CreateModuleTemplateName("test-module", "2.0.0")).
		WithModuleName("test-module").
		WithVersion("2.0.0").
		WithNamespace("kyma-system").
		Build()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(moduleTemplate).
		Build()

	moduleReleaseMeta := builder.NewModuleReleaseMetaBuilder().
		WithModuleName("test-module").
		WithOcmComponentName("kyma-project.io/test-module").
		WithModuleChannelAndVersions([]v1beta2.ChannelVersionAssignment{
			{Channel: "fast", Version: "3.0.0"},
			{Channel: "regular", Version: "2.0.0"},
		}).
		Build()

	lookup := moduletemplateinfolookup.NewLookup(fakeClient)

	result := lookup.Lookup(context.Background(),
		&templatelookup.ModuleInfo{
			Module: v1beta2.Module{
				Name: "test-module",
				// No Channel specified
			},
		},
		&v1beta2.Kyma{
			ObjectMeta: apimetav1.ObjectMeta{
				Namespace: "kyma-system",
			},
			Spec: v1beta2.KymaSpec{
				Channel: "regular",
			},
		},
		moduleReleaseMeta)

	require.NoError(t, result.Err)
	assert.NotNil(t, result.ModuleTemplate)
	assert.Equal(t, "test-module", result.Spec.ModuleName)
	assert.Equal(t, "2.0.0", result.Spec.Version)
	assert.Equal(t, "regular", result.DesiredChannel) // Should fall back to kyma channel
	assert.NotNil(t, result.ComponentId)
}

func TestLookup_UsesDefaultChannelWhenNeitherChannelSpecified(t *testing.T) {
	scheme := machineryruntime.NewScheme()
	require.NoError(t, v1beta2.AddToScheme(scheme))

	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithName(v1beta2.CreateModuleTemplateName("test-module", "1.0.0")).
		WithModuleName("test-module").
		WithVersion("1.0.0").
		WithNamespace("kyma-system").
		Build()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(moduleTemplate).
		Build()

	moduleReleaseMeta := builder.NewModuleReleaseMetaBuilder().
		WithModuleName("test-module").
		WithOcmComponentName("kyma-project.io/test-module").
		WithSingleModuleChannelAndVersions(v1beta2.DefaultChannel, "1.0.0").
		Build()

	lookup := moduletemplateinfolookup.NewLookup(fakeClient)

	result := lookup.Lookup(context.Background(),
		&templatelookup.ModuleInfo{
			Module: v1beta2.Module{
				Name: "test-module",
				// No Channel specified
			},
		},
		&v1beta2.Kyma{
			ObjectMeta: apimetav1.ObjectMeta{
				Namespace: "kyma-system",
			},
			Spec: v1beta2.KymaSpec{
				// No Channel specified
			},
		},
		moduleReleaseMeta)

	require.NoError(t, result.Err)
	assert.NotNil(t, result.ModuleTemplate)
	assert.Equal(t, "test-module", result.Spec.ModuleName)
	assert.Equal(t, "1.0.0", result.Spec.Version)
	assert.Equal(t, v1beta2.DefaultChannel, result.DesiredChannel)
	assert.NotNil(t, result.ComponentId)
}
