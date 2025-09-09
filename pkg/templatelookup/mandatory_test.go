package templatelookup_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/lifecycle-manager/pkg/api"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func TestGetDesiredModuleTemplateForMultipleVersions_ReturnCorrectValue(t *testing.T) {
	firstModuleTemplate := builder.NewModuleTemplateBuilder().
		WithName("warden-1.0.0-dev").
		WithVersion("1.0.0-dev").
		WithLabel("module-diff", "first").
		Build()

	secondModuleTemplate := builder.NewModuleTemplateBuilder().
		WithName("warden-1.0.1-dev").
		WithLabel("module-diff", "second").
		WithAnnotation("operator.kyma-project.io/module-version", "1.0.1-dev").
		Build()

	result, err := templatelookup.GetModuleTemplateWithHigherVersion(firstModuleTemplate, secondModuleTemplate)
	require.NoError(t, err)
	require.Equal(t, secondModuleTemplate, result)
}

func TestGetDesiredModuleTemplateForMultipleVersions_ReturnError_NotSemver(t *testing.T) {
	firstModuleTemplate := builder.NewModuleTemplateBuilder().
		WithName("warden-test").
		WithVersion("test").
		WithLabel("module-diff", "first").
		Build()

	secondModuleTemplate := builder.NewModuleTemplateBuilder().
		WithName("warden-1.0.1-dev").
		WithVersion("1.0.1-dev").
		WithLabel("module-diff", "second").
		Build()

	result, err := templatelookup.GetModuleTemplateWithHigherVersion(firstModuleTemplate, secondModuleTemplate)
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

func TestGetMandatory_OneVersion(t *testing.T) {
	scheme := machineryruntime.NewScheme()
	err := api.AddToScheme(scheme)
	require.NoError(t, err)

	firstModuleTemplate := builder.NewModuleTemplateBuilder().
		WithName("warden-1.0.0").
		WithModuleName("warden").
		WithMandatory(true).
		WithLabel("operator.kyma-project.io/mandatory-module", "true").
		WithVersion("1.0.0").
		Build()

	secondModuleTemplate := builder.NewModuleTemplateBuilder().
		WithName("template-operator-1.0.1").
		WithVersion("1.0.0").
		Build()

	thirdModuleTemplate := builder.NewModuleTemplateBuilder().
		WithName("mandatory-1.0.1").
		WithLabelModuleName("mandatory").
		WithMandatory(true).
		WithLabel("operator.kyma-project.io/mandatory-module", "true").
		WithVersion("1.0.1").
		Build()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(firstModuleTemplate, secondModuleTemplate, thirdModuleTemplate).
		Build()

	result, err := templatelookup.GetMandatory(t.Context(), fakeClient)

	require.NoError(t, err)
	require.Len(t, result, 2)

	require.Contains(t, result, "warden")
	require.Contains(t, result, "mandatory")
	require.Equal(t, result["warden"].Name, firstModuleTemplate.Name)
	require.Equal(t, result["warden"].Spec.Version, firstModuleTemplate.Spec.Version)
	require.NoError(t, result["warden"].Err)
	require.Equal(t, result["mandatory"].Name, thirdModuleTemplate.Name)
	require.Equal(t, result["mandatory"].Spec.Version, thirdModuleTemplate.Spec.Version)
	require.NoError(t, result["mandatory"].Err)
	require.NotContains(t, result, "template-operator")
}

func TestGetMandatory_MultipleVersions(t *testing.T) {
	scheme := machineryruntime.NewScheme()
	err := api.AddToScheme(scheme)
	require.NoError(t, err)

	firstModuleTemplate := builder.NewModuleTemplateBuilder().
		WithName("warden-1.0.0").
		WithModuleName("warden").
		WithMandatory(true).
		WithLabel("operator.kyma-project.io/mandatory-module", "true").
		WithVersion("1.0.0").
		Build()

	secondModuleTemplate := builder.NewModuleTemplateBuilder().
		WithName("template-operator-1.0.1").
		WithVersion("1.0.0").
		Build()

	thirdModuleTemplate := builder.NewModuleTemplateBuilder().
		WithName("mandatory-1.0.1").
		WithLabelModuleName("mandatory").
		WithMandatory(true).
		WithLabel("operator.kyma-project.io/mandatory-module", "true").
		WithVersion("1.0.1").
		Build()

	fourthModuleTemplate := builder.NewModuleTemplateBuilder().
		WithName("warden-1.0.1").
		WithModuleName("warden").
		WithMandatory(true).
		WithLabel("operator.kyma-project.io/mandatory-module", "true").
		WithVersion("1.0.1").
		Build()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(firstModuleTemplate, secondModuleTemplate, thirdModuleTemplate, fourthModuleTemplate).
		Build()

	result, err := templatelookup.GetMandatory(t.Context(), fakeClient)

	require.NoError(t, err)
	require.Len(t, result, 2)

	require.Contains(t, result, "warden")
	require.Contains(t, result, "mandatory")
	require.Equal(t, result["warden"].Name, fourthModuleTemplate.Name)
	require.Equal(t, result["warden"].Spec.Version, fourthModuleTemplate.Spec.Version)
	require.NoError(t, result["warden"].Err)
	require.Equal(t, result["mandatory"].Name, thirdModuleTemplate.Name)
	require.Equal(t, result["mandatory"].Spec.Version, thirdModuleTemplate.Spec.Version)
	require.NoError(t, result["mandatory"].Err)
	require.NotContains(t, result, "template-operator")
}

func TestGetMandatory_WithErrorNotSemVer(t *testing.T) {
	scheme := machineryruntime.NewScheme()
	err := api.AddToScheme(scheme)
	require.NoError(t, err)

	firstModuleTemplate := builder.NewModuleTemplateBuilder().
		WithName("warden-test").
		WithModuleName("warden").
		WithMandatory(true).
		WithLabel("operator.kyma-project.io/mandatory-module", "true").
		WithVersion("test").
		Build()

	secondModuleTemplate := builder.NewModuleTemplateBuilder().
		WithName("warden-1.0.0").
		WithModuleName("warden").
		WithMandatory(true).
		WithLabel("operator.kyma-project.io/mandatory-module", "true").
		WithVersion("1.0.0").
		Build()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(firstModuleTemplate, secondModuleTemplate).
		Build()

	result, err := templatelookup.GetMandatory(t.Context(), fakeClient)

	require.NoError(t, err)
	require.Len(t, result, 1)

	require.Contains(t, result, "warden")
	require.ErrorContains(t, result["warden"].Err, "could not parse version as a semver")
}
