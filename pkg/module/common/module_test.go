package common_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/stretchr/testify/assert"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestApplyDefaultMetaToManifest_WhenCalledWithEmptyKymaName_ReturnsEmptyKymaLabel(t *testing.T) {
	module := createModule()
	kyma := &v1beta2.Kyma{}

	module.ApplyDefaultMetaToManifest(kyma)

	resultLabels := module.GetLabels()
	assert.True(t, resultLabels["operator.kyma-project.io/kyma-name"] == "")
}

func TestApplyDefaultMetaToManifest_WhenCalledWithEmptyKymaName_ReturnsKymaLabelSet(t *testing.T) {
	module := createModule()
	kyma := &v1beta2.Kyma{}
	kyma.SetName("some-kyma-name")

	module.ApplyDefaultMetaToManifest(kyma)

	resultLabels := module.GetLabels()
	assert.True(t, resultLabels["operator.kyma-project.io/kyma-name"] == "some-kyma-name")
}

func TestApplyDefaultMetaToManifest_WhenCalledWithEmptyTemplateLabels_ReturnsWithoutSetControllerName(t *testing.T) {
	module := createModule()
	kyma := &v1beta2.Kyma{}

	module.ApplyDefaultMetaToManifest(kyma)

	resultLabels := module.GetLabels()
	assert.Conta(t, resultLabels)
}

func TestApplyDefaultMetaToManifest_WhenCalledWitNonhEmptyTemplateLabels_ReturnsWithSetControllerName(t *testing.T) {
	module := createModule()
	module.Template.Labels = make(map[string]string)
	module.Template.
	kyma := &v1beta2.Kyma{}

	module.ApplyDefaultMetaToManifest(kyma)

	resultLabels := module.GetLabels()
	assert.NotNil(t, resultLabels)
	assert.Equal(t, resultLabels["operator.kyma-project.io/controller-name"], )
}

func createModule() *common.Module {
	return &common.Module{
		Manifest: &v1beta2.Manifest{
			ObjectMeta: apimetav1.ObjectMeta{},
		},
		Template: &templatelookup.ModuleTemplateInfo{
			ModuleTemplate: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{},
			},
		},
	}
}
