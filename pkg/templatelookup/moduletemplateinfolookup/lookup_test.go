package moduletemplateinfolookup_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func TestLookup_ReturnsError_WhenModuleReleaseMetaIsNil(t *testing.T) {
	lookup := moduletemplateinfolookup.NewLookup(nil)

	result := lookup.Lookup(context.Background(), &templatelookup.ModuleInfo{}, &v1beta2.Kyma{}, nil)

	assert.Error(t, result.Err)
	assert.Equal(t, moduletemplateinfolookup.ErrModuleReleaseMetaRequired, result.Err)
	assert.Nil(t, result.ModuleTemplate)
}

func TestLookup_ReturnsError_WhenGetMandatoryVersionFails(t *testing.T) {
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

	assert.Error(t, result.Err)
	assert.Nil(t, result.ModuleTemplate)
}

func TestLookup_ReturnsError_WhenGetChannelVersionFails(t *testing.T) {
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

	assert.Error(t, result.Err)
	assert.Nil(t, result.ModuleTemplate)
}

func TestLookup_ReturnsError_WhenOcmComponentNameIsInvalid(t *testing.T) {
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

	assert.Error(t, result.Err)
	assert.Nil(t, result.ModuleTemplate)
}

func TestLookup_ReturnsError_WhenModuleTemplateNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
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

	assert.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "failed to get module template")
	assert.Nil(t, result.ModuleTemplate)
}

func TestLookup_Success_WithMandatoryModule(t *testing.T) {
	scheme := runtime.NewScheme()
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

	assert.NoError(t, result.Err)
	assert.NotNil(t, result.ModuleTemplate)
	assert.Equal(t, "test-module", result.ModuleTemplate.Spec.ModuleName)
	assert.Equal(t, "1.0.0", result.ModuleTemplate.Spec.Version)
	assert.Equal(t, "regular", result.DesiredChannel)
	assert.NotNil(t, result.ComponentId)
}

func TestLookup_Success_WithChannelBasedModule(t *testing.T) {
	scheme := runtime.NewScheme()
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

	assert.NoError(t, result.Err)
	assert.NotNil(t, result.ModuleTemplate)
	assert.Equal(t, "test-module", result.ModuleTemplate.Spec.ModuleName)
	assert.Equal(t, "2.0.0", result.ModuleTemplate.Spec.Version)
	assert.Equal(t, "regular", result.DesiredChannel)
	assert.NotNil(t, result.ComponentId)
}

func TestLookup_UsesModuleChannelOverKymaChannel(t *testing.T) {
	scheme := runtime.NewScheme()
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

	assert.NoError(t, result.Err)
	assert.NotNil(t, result.ModuleTemplate)
	assert.Equal(t, "test-module", result.ModuleTemplate.Spec.ModuleName)
	assert.Equal(t, "3.0.0", result.ModuleTemplate.Spec.Version)
	assert.Equal(t, "fast", result.DesiredChannel) // Should use module channel, not kyma channel
	assert.NotNil(t, result.ComponentId)
}

func TestLookup_FallsBackToKymaChannelWhenModuleChannelEmpty(t *testing.T) {
	scheme := runtime.NewScheme()
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

	assert.NoError(t, result.Err)
	assert.NotNil(t, result.ModuleTemplate)
	assert.Equal(t, "test-module", result.ModuleTemplate.Spec.ModuleName)
	assert.Equal(t, "2.0.0", result.ModuleTemplate.Spec.Version)
	assert.Equal(t, "regular", result.DesiredChannel) // Should fall back to kyma channel
	assert.NotNil(t, result.ComponentId)
}

func TestLookup_UsesDefaultChannelWhenNeitherChannelSpecified(t *testing.T) {
	scheme := runtime.NewScheme()
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

	assert.NoError(t, result.Err)
	assert.NotNil(t, result.ModuleTemplate)
	assert.Equal(t, "test-module", result.ModuleTemplate.Spec.ModuleName)
	assert.Equal(t, "1.0.0", result.ModuleTemplate.Spec.Version)
	assert.Equal(t, v1beta2.DefaultChannel, result.DesiredChannel)
	assert.NotNil(t, result.ComponentId)
}
