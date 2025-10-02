package generator_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/status/modules/generator"
	"github.com/kyma-project/lifecycle-manager/internal/templatelookup"
	modulecommon "github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func TestGenerateModuleStatus_WhenCalledWithNilTemplateInfo_ReturnsError(t *testing.T) {
	module := &modulecommon.Module{TemplateInfo: nil}

	statusGenerator := generator.NewModuleStatusGenerator(noOpGenerateFromError)
	_, err := statusGenerator.GenerateModuleStatus(module, &v1beta2.ModuleStatus{})

	require.Error(t, err)
	require.ErrorIs(t, err, generator.ErrModuleNeedsTemplateInfo)
}

func TestGenerateModuleStatus_WhenCalledWithNilTemplateErrorAndNilModuleTemplate_ReturnsError(t *testing.T) {
	module := &modulecommon.Module{
		TemplateInfo: &templatelookup.ModuleTemplateInfo{
			Err:            nil,
			ModuleTemplate: nil,
		},
	}

	statusGenerator := generator.NewModuleStatusGenerator(noOpGenerateFromError)
	_, err := statusGenerator.GenerateModuleStatus(module, &v1beta2.ModuleStatus{})

	require.Error(t, err)
	require.ErrorIs(t, err, generator.ErrModuleNeedsTemplateErrorOrTemplate)
}

func TestGenerateModuleStatus_WhenCalledWithErrorInTemplate_CallsGenerateFromErrorFunc(t *testing.T) {
	module := &modulecommon.Module{
		TemplateInfo: &templatelookup.ModuleTemplateInfo{
			Err:            errors.New("some template error"),
			ModuleTemplate: createModuleTemplate(),
		},
	}

	generateFromErrorFuncStub := func(_ error, _, _, _ string, _ *v1beta2.ModuleStatus) (*v1beta2.ModuleStatus, error) {
		return &v1beta2.ModuleStatus{
			Name: "stub status",
		}, nil
	}

	statusGenerator := generator.NewModuleStatusGenerator(generateFromErrorFuncStub)
	result, err := statusGenerator.GenerateModuleStatus(module, &v1beta2.ModuleStatus{})

	require.NoError(t, err)
	assert.Equal(t, "stub status", result.Name)
}

func TestGenerateModuleStatus_WhenCalledWithErrorInTemplateAndFuncReturnsError_ReturnsError(t *testing.T) {
	module := &modulecommon.Module{
		TemplateInfo: &templatelookup.ModuleTemplateInfo{
			Err:            errors.New("some template error"),
			ModuleTemplate: createModuleTemplate(),
		},
	}
	expectedErr := errors.New("generator error")
	generateFromErrorFuncStub := func(_ error, _, _, _ string, _ *v1beta2.ModuleStatus) (*v1beta2.ModuleStatus, error) {
		return nil, expectedErr
	}

	statusGenerator := generator.NewModuleStatusGenerator(generateFromErrorFuncStub)
	_, err := statusGenerator.GenerateModuleStatus(module, &v1beta2.ModuleStatus{})

	require.ErrorIs(t, err, expectedErr)
}

func TestGenerateModuleStatus_WhenCalledWithNilManifest_ReturnsError(t *testing.T) {
	module := &modulecommon.Module{
		TemplateInfo: &templatelookup.ModuleTemplateInfo{
			ModuleTemplate: createModuleTemplate(),
		},
	}

	statusGenerator := generator.NewModuleStatusGenerator(noOpGenerateFromError)
	_, err := statusGenerator.GenerateModuleStatus(module, &v1beta2.ModuleStatus{})

	require.Error(t, err)
	require.ErrorIs(t, err, generator.ErrModuleNeedsManifest)
}

func TestGenerateModuleStatus_WhenCalledWithManifestNilSpec_ReturnsError(t *testing.T) {
	module := &modulecommon.Module{
		TemplateInfo: &templatelookup.ModuleTemplateInfo{},
		Manifest:     &v1beta2.Manifest{},
	}

	statusGenerator := generator.NewModuleStatusGenerator(noOpGenerateFromError)
	_, err := statusGenerator.GenerateModuleStatus(module, &v1beta2.ModuleStatus{})

	require.Error(t, err)
}

func TestGenerateModuleStatus_WhenCalledWithTemplateAndManifest_CreatesMinimalModuleStatus(t *testing.T) {
	module := createModule()

	statusGenerator := generator.NewModuleStatusGenerator(noOpGenerateFromError)
	result, err := statusGenerator.GenerateModuleStatus(module, &v1beta2.ModuleStatus{})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, module.ModuleName, result.Name)
	assert.Equal(t, module.FQDN, result.FQDN)
	assert.Equal(t, shared.State("test-state"), result.State)
	assert.Equal(t, "test-channel", result.Channel)

	assert.NotNil(t, result.Manifest)
	assert.Equal(t, "test-manifest", result.Manifest.Name)
	assert.Equal(t, "test-namespace", result.Manifest.Namespace)
	assert.Equal(t, int64(123), result.Manifest.Generation)
	assert.Equal(t, "Manifest", result.Manifest.Kind)
	assert.Equal(t, "operator.kyma-project.io/v1beta2", result.Manifest.APIVersion)

	assert.Equal(t, "test-template", result.Template.Name)
	assert.Equal(t, "test-namespace", result.Template.Namespace)
	assert.Equal(t, int64(123), result.Template.Generation)
	assert.Equal(t, "ModuleTemplate", result.Template.Kind)
	assert.Equal(t, "operator.kyma-project.io/v1beta2", result.Template.APIVersion)
}

func TestGenerateModuleStatus_WhenCalledWithManifestResource_CreatesTrackingObjectForResource(t *testing.T) {
	module := createModule()
	module.Manifest = createManifestWithResource()
	module.Manifest.Annotations = make(map[string]string)
	module.Manifest.Spec.CustomResourcePolicy = v1beta2.CustomResourcePolicyCreateAndDelete

	statusGenerator := generator.NewModuleStatusGenerator(noOpGenerateFromError)
	result, err := statusGenerator.GenerateModuleStatus(module, &v1beta2.ModuleStatus{})

	require.NoError(t, err)
	assert.NotNil(t, result)

	assert.NotNil(t, result.Resource)
	assert.Equal(t, "test-kind", result.Resource.Kind)
	assert.Equal(t, "test-version", result.Resource.APIVersion)
	assert.Equal(t, "test-manifest-resource", result.Resource.Name)
	assert.Equal(t, "test-namespace", result.Resource.Namespace)
	assert.Equal(t, int64(123), result.Resource.Generation)
}

func TestGenerateModuleStatus_WhenCalledWithIsClusterScopedAnnotation_RemovesResourceNamespace(t *testing.T) {
	module := createModule()
	module.TemplateInfo.ModuleTemplate = createClusterScopedModuleTemplate()
	module.Manifest = createManifestWithResource()
	module.Manifest.Annotations = make(map[string]string)
	module.Manifest.Spec.CustomResourcePolicy = v1beta2.CustomResourcePolicyCreateAndDelete

	statusGenerator := generator.NewModuleStatusGenerator(noOpGenerateFromError)
	result, err := statusGenerator.GenerateModuleStatus(module, &v1beta2.ModuleStatus{})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Resource)

	assert.Empty(t, result.Resource.Namespace)
}

func TestGenerateModuleStatus_WhenModuleIsUnmanaged_StateIsUnmanagedAndTrackingObjectsNil(t *testing.T) {
	module := createModule()
	module.IsUnmanaged = true

	statusGenerator := generator.NewModuleStatusGenerator(noOpGenerateFromError)
	result, err := statusGenerator.GenerateModuleStatus(module, &v1beta2.ModuleStatus{})

	require.NoError(t, err)
	assert.NotNil(t, result)

	assert.Equal(t, shared.StateUnmanaged, result.State)
	assert.Nil(t, result.Template)
	assert.Nil(t, result.Manifest)
	assert.Nil(t, result.Resource)
}

// Resource creator helper functions

func createModule() *modulecommon.Module {
	return &modulecommon.Module{
		ModuleName: "test-module",
		FQDN:       "test-fqdn",
		TemplateInfo: &templatelookup.ModuleTemplateInfo{
			DesiredChannel: "test-channel",
			ModuleTemplate: createModuleTemplate(),
		},
		Manifest: createManifest(),
	}
}

func createManifest() *v1beta2.Manifest {
	return builder.NewManifestBuilder().
		WithName("test-manifest").
		WithNamespace("test-namespace").
		WithGeneration(123).
		WithStatus(shared.Status{
			State: "test-state",
		}).
		Build()
}

func createManifestWithResource() *v1beta2.Manifest {
	manifest := createManifest()
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "test-version",
			"kind":       "test-kind",
			"metadata": map[string]interface{}{
				"name":       "test-manifest-resource",
				"namespace":  "test-namespace",
				"generation": "123",
			},
		},
	}

	_ = unstructured.SetNestedField(resource.Object, int64(123), "metadata", "generation")
	manifest.Spec.Resource = resource
	return manifest
}

func createModuleTemplate() *v1beta2.ModuleTemplate {
	return builder.NewModuleTemplateBuilder().
		WithName("test-template").
		WithNamespace("test-namespace").
		WithGeneration(123).
		Build()
}

func createClusterScopedModuleTemplate() *v1beta2.ModuleTemplate {
	return builder.NewModuleTemplateBuilder().
		WithName("test-template").
		WithNamespace("test-namespace").
		WithGeneration(123).
		WithAnnotation(shared.IsClusterScopedAnnotation, "true").
		Build()
}

var noOpGenerateFromError = func(_ error, _, _, _ string, _ *v1beta2.ModuleStatus) (*v1beta2.ModuleStatus, error) {
	return nil, nil
}
