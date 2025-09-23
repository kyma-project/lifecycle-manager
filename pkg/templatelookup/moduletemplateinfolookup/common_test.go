package moduletemplateinfolookup_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func Test_TemplateNameMatch_WhenModuleNameFieldIsMatching(t *testing.T) {
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithModuleName("test-module").
		Build()

	isNameMatching := moduletemplateinfolookup.TemplateNameMatch(moduleTemplate, "test-module")
	assert.True(t, isNameMatching)
}

func Test_TemplateNameMatch_WhenModuleNameFieldIsNotMatching(t *testing.T) {
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithModuleName("test-module").
		Build()

	isNameMatching := moduletemplateinfolookup.TemplateNameMatch(moduleTemplate, "module")
	assert.False(t, isNameMatching)
}

func Test_TemplateNameMatch_WhenNoModuleNameFieldButMatchingLabel(t *testing.T) {
	moduleTemplate := builder.NewModuleTemplateBuilder().
		Build()

	isNameMatching := moduletemplateinfolookup.TemplateNameMatch(moduleTemplate, "test-module")
	assert.True(t, isNameMatching)
}

func Test_TemplateNameMatch_WhenNoModuleNameFieldAndNoMatchingLabel(t *testing.T) {
	moduleTemplate := builder.NewModuleTemplateBuilder().
		Build()

	isNameMatching := moduletemplateinfolookup.TemplateNameMatch(moduleTemplate, "test-module")
	assert.False(t, isNameMatching)
}
