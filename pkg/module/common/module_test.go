package common_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestApplyDefaultMetaToManifest_WhenCalledWithEmptyKymaName_ReturnsEmptyKymaLabel(t *testing.T) {
	module := createModule()
	kyma := &v1beta2.Kyma{}

	module.ApplyDefaultMetaToManifest(kyma)

	resultLabels := module.GetLabels()
	assert.Equal(t, resultLabels["operator.kyma-project.io/kyma-name"], "")
}

func TestApplyDefaultMetaToManifest_WhenCalledWithEmptyKymaName_ReturnsKymaLabelSet(t *testing.T) {
	module := createModule()
	kyma := &v1beta2.Kyma{}
	kyma.SetName("some-kyma-name")

	module.ApplyDefaultMetaToManifest(kyma)

	resultLabels := module.GetLabels()
	assert.Equal(t, resultLabels["operator.kyma-project.io/kyma-name"], "some-kyma-name")
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
