package common_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	modulecommon "github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
)

func TestApplyDefaultMetaToManifest_WhenCalledWithEmptyKymaName_ReturnsEmptyKymaLabel(t *testing.T) {
	module := createModule()
	kyma := &v1beta2.Kyma{}

	module.ApplyDefaultMetaToManifest(kyma)

	resultLabels := module.Manifest.GetLabels()
	assert.Empty(t, resultLabels["operator.kyma-project.io/kyma-name"])
}

func TestApplyDefaultMetaToManifest_WhenCalledWithEmptyKymaName_ReturnsKymaLabelSet(t *testing.T) {
	module := createModule()
	kyma := &v1beta2.Kyma{}
	kyma.SetName("some-kyma-name")

	module.ApplyDefaultMetaToManifest(kyma)

	resultLabels := module.Manifest.GetLabels()
	assert.Equal(t, "some-kyma-name", resultLabels["operator.kyma-project.io/kyma-name"])
}

func TestApplyDefaultMetaToManifest_WhenCalledWithMandatoryModule_SetsMandatoryModuleLabel(t *testing.T) {
	module := createModule()
	module.TemplateInfo.Spec.Mandatory = true
	kyma := &v1beta2.Kyma{}

	module.ApplyDefaultMetaToManifest(kyma)

	resultLabels := module.Manifest.GetLabels()
	assert.Equal(t, "true", resultLabels["operator.kyma-project.io/mandatory-module"])
}

func TestApplyDefaultMetaToManifest_WhenCalledWithControllerName_SetsControllerNameLabel(t *testing.T) {
	module := createModule()
	module.TemplateInfo.SetLabels(map[string]string{"operator.kyma-project.io/controller-name": "some-controller"})
	kyma := &v1beta2.Kyma{}

	module.ApplyDefaultMetaToManifest(kyma)

	resultLabels := module.Manifest.GetLabels()
	assert.Equal(t, "some-controller", resultLabels["operator.kyma-project.io/controller-name"])
}

func TestApplyDefaultMetaToManifest_WhenCalledWithMandatoryModule_NoChannelLabelIsSet(t *testing.T) {
	module := createModule()
	module.TemplateInfo.Spec.Mandatory = true
	kyma := &v1beta2.Kyma{}

	module.ApplyDefaultMetaToManifest(kyma)

	resultLabels := module.Manifest.GetLabels()
	assert.NotContains(t, resultLabels, "operator.kyma-project.io/channel")
}

func TestApplyDefaultMetaToManifest_WhenCalledWithNonMandatoryModule_ChannelLabelIsSet(t *testing.T) {
	module := createModule()
	module.TemplateInfo.DesiredChannel = "regular"
	kyma := &v1beta2.Kyma{}

	module.ApplyDefaultMetaToManifest(kyma)

	resultLabels := module.Manifest.GetLabels()
	assert.Equal(t, "regular", resultLabels["operator.kyma-project.io/channel"])
}

func TestApplyDefaultMetaToManifest_WhenCalled_SetsManagedByLabel(t *testing.T) {
	module := createModule()
	kyma := &v1beta2.Kyma{}

	module.ApplyDefaultMetaToManifest(kyma)

	resultLabels := module.Manifest.GetLabels()
	assert.Equal(t, "lifecycle-manager", resultLabels["operator.kyma-project.io/managed-by"])
}

func TestApplyDefaultMetaToManifest_WhenCalled_SetsOCMAnnotation(t *testing.T) {
	module := createModule()
	module.OCMComponentName = "example.org/some-module/backend"
	kyma := &v1beta2.Kyma{}

	module.ApplyDefaultMetaToManifest(kyma)

	resultAnnotations := module.Manifest.GetAnnotations()
	assert.Equal(t, "example.org/some-module/backend", resultAnnotations["operator.kyma-project.io/ocm-component-name"])
	assert.Equal(t, "example.org/some-module/backend", resultAnnotations["operator.kyma-project.io/fqdn"])
}

func TestApplyDefaultMetaToManifest_WhenCalledWithUnmanaged_SetsUnmanagedAnnotation(t *testing.T) {
	module := createModule()
	module.IsUnmanaged = true
	kyma := &v1beta2.Kyma{}

	module.ApplyDefaultMetaToManifest(kyma)

	resultAnnotations := module.Manifest.GetAnnotations()
	assert.Equal(t, "true", resultAnnotations["operator.kyma-project.io/is-unmanaged"])
}

func createModule() *modulecommon.Module {
	return &modulecommon.Module{
		Manifest: &v1beta2.Manifest{
			ObjectMeta: apimetav1.ObjectMeta{},
		},
		TemplateInfo: &templatelookup.ModuleTemplateInfo{
			ModuleTemplate: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{},
			},
		},
	}
}

func TestCreateModuleName(t *testing.T) {
	tests := []struct {
		testName      string
		componentName string
		prefix        string
		moduleName    string
		expected      string
	}{
		{
			testName:      "Standard component name",
			componentName: "kyma-project.io/module/some-module",
			prefix:        "default-id",
			moduleName:    "module1",
			expected:      "default-id-some-module-1831772875",
		},
		{
			testName:      "Short component name",
			componentName: "example.org/some-module",
			prefix:        "default-id",
			moduleName:    "module1",
			expected:      "default-id-some-module-1457091198",
		},
		{
			testName:      "Very long component name",
			componentName: "example.org/this-is-a-very-long-component-name-that-exceeds-the-maximum-length",
			prefix:        "default-id",
			moduleName:    "module1",
			expected:      "default-id-this-is-a-very-long-component-name-that-exceeds-the",
		},
		{
			testName:      "Component name with invalid structure",
			componentName: "simple-component",
			prefix:        "default-id",
			moduleName:    "module1",
			expected:      "default-id-simple-component-3590401810",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			result := modulecommon.CreateModuleName(tt.componentName, tt.prefix, tt.moduleName)
			assert.Equal(t, tt.expected, result)
		})
	}
}
