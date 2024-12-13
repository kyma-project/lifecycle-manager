package templatelookup_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func TestGetDesiredModuleTemplateForMultipleVersions_ReturnCorrectValue(t *testing.T) {
	firstModuleTemplate := builder.NewModuleTemplateBuilder().
		WithName("warden").
		WithVersion("1.0.0-dev").
		WithLabel("module-diff", "first").
		Build()

	secondModuleTemplate := builder.NewModuleTemplateBuilder().
		WithName("warden").
		WithVersion("1.0.1-dev").
		WithLabel("module-diff", "second").
		Build()

	result, err := templatelookup.GetDesiredModuleTemplateForMultipleVersions(firstModuleTemplate, secondModuleTemplate)
	require.NoError(t, err)
	require.Equal(t, secondModuleTemplate, result)
}

func TestGetDesiredModuleTemplateForMultipleVersions_ReturnError_NotSemver(t *testing.T) {
	firstModuleTemplate := builder.NewModuleTemplateBuilder().
		WithName("warden").
		WithVersion("test").
		WithLabel("module-diff", "first").
		Build()

	secondModuleTemplate := builder.NewModuleTemplateBuilder().
		WithName("warden").
		WithVersion("1.0.1-dev").
		WithLabel("module-diff", "second").
		Build()

	result, err := templatelookup.GetDesiredModuleTemplateForMultipleVersions(firstModuleTemplate, secondModuleTemplate)
	require.ErrorContains(t, err, "could not parse version as a semver")
	require.Nil(t, result)
}

func TestGetModuleName_withModuleName(t *testing.T) {
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithModuleName("warden").
		WithLabelModuleName("warden-dev").
		Build()

	result := templatelookup.GetModuleName(moduleTemplate)
	require.Equal(t, "warden", result)
}

func TestGetModuleName_withModuleNameLabel(t *testing.T) {
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithModuleName("").
		WithLabelModuleName("warden").
		Build()

	result := templatelookup.GetModuleName(moduleTemplate)
	require.Equal(t, "warden", result)
}

func TestGetModuleSemverVersion_WithCorrectSemVer_SpecVersion(t *testing.T) {
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithVersion("1.0.0-dev").
		Build()

	result, err := templatelookup.GetModuleSemverVersion(moduleTemplate)
	require.NoError(t, err)
	require.Equal(t, "1.0.0-dev", result.String())
}

func TestGetModuleSemverVersion_WithCorrectSemVer_VersionAnnotation(t *testing.T) {
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithAnnotation("operator.kyma-project.io/module-version", "1.0.0-dev").
		Build()

	result, err := templatelookup.GetModuleSemverVersion(moduleTemplate)
	require.NoError(t, err)
	require.Equal(t, "1.0.0-dev", result.String())
}

func TestGetModuleSemverVersion_ReturnError_NotSemver_SpecVersion(t *testing.T) {
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithVersion("dev").
		Build()

	result, err := templatelookup.GetModuleSemverVersion(moduleTemplate)
	require.ErrorContains(t, err, "could not parse version as a semver")
	require.Nil(t, result)
}

func TestGetModuleSemverVersion_ReturnError_NotSemver_VersionAnnotation(t *testing.T) {
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithAnnotation("operator.kyma-project.io/module-version", "dev").
		Build()

	result, err := templatelookup.GetModuleSemverVersion(moduleTemplate)
	require.ErrorContains(t, err, "could not parse version as a semver")
	require.Nil(t, result)
}
