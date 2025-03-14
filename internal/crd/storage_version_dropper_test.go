package crd_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/lifecycle-manager/internal/crd"
)

func TestParseStorageVersionsMap(t *testing.T) {
	versions := "Manifest:v1beta1,Watcher:v1beta1,ModuleTemplate:v1beta1,Kyma:v1beta1"

	expectedOutput := map[string]string{
		"Manifest":       "v1beta1",
		"Watcher":        "v1beta1",
		"ModuleTemplate": "v1beta1",
		"Kyma":           "v1beta1",
	}
	assert.Equalf(t, expectedOutput, crd.ParseStorageVersionsMap(versions), "parseStorageVersionsMap(%v)",
		versions)
}

func TestDropStoredVersion(t *testing.T) {
	versionToBeDropped := "Manifest:v1beta1"
	currentCrds := []machineryruntime.Object{
		&apiextensionsv1.CustomResourceDefinitionList{
			Items: []apiextensionsv1.CustomResourceDefinition{
				{
					ObjectMeta: apimetav1.ObjectMeta{Name: "Manifest"},
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Kind: "Manifest",
						},
						Group: "operator.kyma-project.io",
					},
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						StoredVersions: []string{"v1beta1", "v1beta2"},
					},
				},
				{
					ObjectMeta: apimetav1.ObjectMeta{Name: "Test"},
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Names: apiextensionsv1.CustomResourceDefinitionNames{
							Kind: "ModuleTemplate",
						},
						Group: "operator.kyma-project.io",
					},
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						StoredVersions: []string{"v1beta1"},
					},
				},
			},
		},
	}

	scheme := machineryruntime.NewScheme()
	_ = apiextensionsv1.AddToScheme(scheme)
	fakeClientBuilder := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(currentCrds...).Build()
	crd.DropStoredVersion(t.Context(), fakeClientBuilder, versionToBeDropped)

	var updatedCRD apiextensionsv1.CustomResourceDefinition
	err := fakeClientBuilder.Get(t.Context(), client.ObjectKey{Name: "Manifest"}, &updatedCRD)
	require.NoError(t, err, "error getting updated CustomResourceDefinition")

	expectedStatus := apiextensionsv1.CustomResourceDefinitionStatus{
		StoredVersions: []string{"v1beta2"},
	}
	require.Equal(t, expectedStatus, updatedCRD.Status, "status should be updated")
}
