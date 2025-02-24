package moduletemplateinfolookup_test

import (
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_TemplateNameMatch_WhenModuleNameFieldIsMatching(t *testing.T) {
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithName("test-module-regular").
		WithModuleName("test-module").
		WithVersion("").
		WithChannel("regular").
		Build()

	isNameMatching := moduletemplateinfolookup.TemplateNameMatch(moduleTemplate, "test-module")
	assert.True(t, isNameMatching)
}

func Test_TemplateNameMatch_WhenModuleNameFieldIsNotMatching(t *testing.T) {
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithName("test-module-regular").
		WithModuleName("test-module").
		WithVersion("").
		WithChannel("regular").
		Build()

	isNameMatching := moduletemplateinfolookup.TemplateNameMatch(moduleTemplate, "module")
	assert.False(t, isNameMatching)
}

func Test_TemplateNameMatch_WhenNoModuleNameFieldButMatchingLabel(t *testing.T) {
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithName("test-module-regular").
		WithLabelModuleName("test-module").
		WithVersion("").
		WithChannel("regular").
		Build()

	isNameMatching := moduletemplateinfolookup.TemplateNameMatch(moduleTemplate, "test-module")
	assert.True(t, isNameMatching)
}

func Test_TemplateNameMatch_WhenNoModuleNameFieldAndNoMatchingLabel(t *testing.T) {
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithName("test-module-regular").
		WithVersion("").
		WithChannel("regular").
		Build()

	isNameMatching := moduletemplateinfolookup.TemplateNameMatch(moduleTemplate, "test-module")
	assert.False(t, isNameMatching)
}
