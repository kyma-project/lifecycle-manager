package modulecr_test

import (
	"testing"

	templatev1alpha1 "github.com/kyma-project/template-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/finalizer"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/modulecr"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

func TestClient_RemoveDefaultModuleCR(t *testing.T) {
	// Given a manifest CR with finalizer and a resource CR deployed in the cluster
	testScheme := machineryruntime.NewScheme()
	err := v1beta2.AddToScheme(testScheme)
	require.NoError(t, err)

	kcpClient := fake.NewClientBuilder().WithScheme(testScheme).WithRESTMapper(getRestMapper()).Build()
	skrClient := modulecr.NewClient(kcpClient)

	manifest := testutils.NewTestManifest("test-manifest")
	manifest.SetFinalizers([]string{finalizer.CustomResourceManagerFinalizer, finalizer.DefaultFinalizer})
	moduleCR := unstructured.Unstructured{}
	moduleCR.SetGroupVersionKind(
		schema.GroupVersionKind{
			Group:   templatev1alpha1.GroupVersion.Group,
			Version: templatev1alpha1.GroupVersion.Version,
			Kind:    string(templatev1alpha1.SampleKind),
		},
	)
	const moduleName = "test-resource"
	moduleCR.SetName(moduleName)
	moduleCR.SetNamespace(shared.DefaultRemoteNamespace)
	manifest.Spec.Resource = &moduleCR
	err = kcpClient.Create(t.Context(), manifest.Spec.Resource)
	require.NoError(t, err)

	err = kcpClient.Create(t.Context(), manifest)
	require.NoError(t, err)

	// When Manifest CR is under deletion, fakeClient does not support setting deletionTimestamp
	deletionTimestamp := apimetav1.Now()
	manifest.SetDeletionTimestamp(&deletionTimestamp)

	// And deleting the resource CR
	err = skrClient.RemoveDefaultModuleCR(t.Context(), kcpClient, manifest)
	require.NoError(t, err)

	// And in second deletion attempt, the resource should not be found and the finalizer should be removed
	err = skrClient.RemoveDefaultModuleCR(t.Context(), kcpClient, manifest)
	require.ErrorIs(t, err, finalizer.ErrRequeueRequired)

	// Then the resource CR should be deleted
	err = kcpClient.Get(t.Context(), client.ObjectKey{Name: moduleCR.GetName(), Namespace: moduleCR.GetNamespace()},
		&moduleCR)
	require.True(t, apierrors.IsNotFound(err))

	// Then the finalizer should be removed
	err = kcpClient.Get(t.Context(), client.ObjectKey{Name: manifest.GetName(), Namespace: manifest.GetNamespace()},
		manifest)
	require.NoError(t, err)
	assert.NotContains(t, manifest.GetFinalizers(), finalizer.CustomResourceManagerFinalizer)
}

func TestClient_SyncDefaultModuleCR(t *testing.T) {
	// Given a manifest CR with a resource CR
	testScheme := machineryruntime.NewScheme()
	err := v1beta2.AddToScheme(testScheme)
	require.NoError(t, err)

	kcpClient := fake.NewClientBuilder().WithScheme(testScheme).Build()
	skrClient := modulecr.NewClient(kcpClient)
	manifest := testutils.NewTestManifest("test-manifest")
	manifest.SetFinalizers([]string{finalizer.CustomResourceManagerFinalizer, finalizer.DefaultFinalizer})
	moduleCR := unstructured.Unstructured{}
	moduleCR.SetGroupVersionKind(
		schema.GroupVersionKind{
			Group:   templatev1alpha1.GroupVersion.Group,
			Version: templatev1alpha1.GroupVersion.Version,
			Kind:    string(templatev1alpha1.SampleKind),
		},
	)
	const moduleName = "test-resource"
	moduleCR.SetName(moduleName)
	moduleCR.SetNamespace(shared.DefaultRemoteNamespace)
	manifest.Spec.Resource = &moduleCR

	// When syncing the module CR
	err = skrClient.SyncDefaultModuleCR(t.Context(), manifest)
	require.NoError(t, err)

	// Then the resource CR should be created
	resource := &unstructured.Unstructured{}
	resource.SetGroupVersionKind(manifest.Spec.Resource.GroupVersionKind())
	err = skrClient.Get(t.Context(), client.ObjectKey{Name: moduleName, Namespace: shared.DefaultRemoteNamespace},
		resource)
	require.NoError(t, err)
	// And the resource should have the managed-by label
	labels := resource.GetLabels()
	assert.Equal(t, shared.ManagedByLabelValue, labels[shared.ManagedBy])

	// When the resource is deleted
	err = kcpClient.Delete(t.Context(), resource)
	require.NoError(t, err)

	// And syncing again, it should recreate the resource
	err = skrClient.SyncDefaultModuleCR(t.Context(), manifest)
	require.NoError(t, err)

	err = skrClient.Get(t.Context(), client.ObjectKey{Name: moduleName, Namespace: shared.DefaultRemoteNamespace},
		resource)
	require.NoError(t, err)
}

func TestClient_GetAllModuleCRsExcludingDefaultCR_WithCreateAndDeletePolicy(t *testing.T) {
	// Given a manifest CR and two resource CRs deployed in the cluster
	testScheme := machineryruntime.NewScheme()
	err := v1beta2.AddToScheme(testScheme)
	require.NoError(t, err)

	kcpClient := fake.NewClientBuilder().WithScheme(testScheme).WithRESTMapper(getRestMapper()).Build()
	skrClient := modulecr.NewClient(kcpClient)

	manifest := testutils.NewTestManifest("test-manifest")
	moduleCR := unstructured.Unstructured{}
	moduleCR.SetGroupVersionKind(
		schema.GroupVersionKind{
			Group:   shared.OperatorGroup,
			Version: string(templatev1alpha1.Version),
			Kind:    string(templatev1alpha1.SampleKind),
		},
	)
	const moduleName = "test-resource"
	moduleCR.SetName(moduleName)
	moduleCR.SetNamespace(shared.DefaultRemoteNamespace)
	manifest.Spec.Resource = &moduleCR
	err = kcpClient.Create(t.Context(), manifest.Spec.Resource)
	require.NoError(t, err)

	err = kcpClient.Create(t.Context(), manifest)
	require.NoError(t, err)

	const moduleName2 = "test-resource-2"
	moduleCR2 := unstructured.Unstructured{}
	moduleCR2.SetName(moduleName2)
	moduleCR2.SetNamespace(shared.DefaultRemoteNamespace)
	moduleCR2.SetGroupVersionKind(moduleCR.GroupVersionKind())
	err = skrClient.Create(t.Context(), &moduleCR2)
	require.NoError(t, err)

	// When Getting all Module CRs excluding the default CR
	moduleCRs, err := skrClient.GetAllModuleCRsExcludingDefaultCR(t.Context(), manifest)
	require.NoError(t, err)
	require.Len(t, moduleCRs, 1)

	// Then the non-default module CR should be returned
	assert.Equal(t, moduleCRs[0].GetName(), moduleCR2.GetName())
	assert.Equal(t, moduleCRs[0].GetNamespace(), moduleCR2.GetNamespace())
}

func TestClient_GetAllModuleCRsExcludingDefaultCR_ReturnsEmpty_WhenModuleCRApiVersionUpgrade(t *testing.T) {
	// Given a manifest CR and two resource CRs deployed in the cluster
	testScheme := machineryruntime.NewScheme()
	err := v1beta2.AddToScheme(testScheme)
	require.NoError(t, err)

<<<<<<< HEAD
	kcpClient := fake.NewClientBuilder().WithScheme(testScheme).WithRESTMapper(getRestMapper()).Build()
	skrClient := modulecr.NewClient(kcpClient)

	manifest := testutils.NewTestManifest("test-manifest")
	newerModuleCR := unstructured.Unstructured{}
	newerModuleCR.SetGroupVersionKind(
		schema.GroupVersionKind{
			Group:   shared.OperatorGroup,
			Version: "v1beta1",
			Kind:    string(templatev1alpha1.SampleKind),
		},
	)
	const moduleName = "test-resource"
	newerModuleCR.SetName(moduleName)
	newerModuleCR.SetNamespace(shared.DefaultRemoteNamespace)
	manifest.Spec.Resource = &newerModuleCR
	err = kcpClient.Create(t.Context(), manifest.Spec.Resource)
	require.NoError(t, err)

	err = kcpClient.Create(t.Context(), manifest)
	require.NoError(t, err)

	existingModuleCRInSkr := unstructured.Unstructured{}
	existingModuleCRInSkr.SetName(moduleName)
	existingModuleCRInSkr.SetNamespace(shared.DefaultRemoteNamespace)
	existingModuleCRInSkr.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   shared.OperatorGroup,
		Version: "v1alpha1",
		Kind:    string(templatev1alpha1.SampleKind),
	})
	err = skrClient.Create(t.Context(), &existingModuleCRInSkr)
	require.NoError(t, err)

	// When Getting all Module CRs excluding the default CR
	moduleCRs, err := skrClient.GetAllModuleCRsExcludingDefaultCR(t.Context(), manifest)
	require.NoError(t, err)
	require.Empty(t, moduleCRs)
}

func TestClient_GetAllModuleCRsExcludingDefaultCR_WithIgnorePolicy(t *testing.T) {
	// Given a manifest CR and two resource CRs deployed in the cluster
	testScheme := machineryruntime.NewScheme()
	err := v1beta2.AddToScheme(testScheme)
	require.NoError(t, err)

	kcpClient := fake.NewClientBuilder().WithScheme(testScheme).WithRESTMapper(getRestMapper()).Build()
	skrClient := modulecr.NewClient(kcpClient)

	manifest := testutils.NewTestManifest("test-manifest")
	manifest.Spec.CustomResourcePolicy = v1beta2.CustomResourcePolicyIgnore
	moduleCR := unstructured.Unstructured{}
	moduleCR.SetGroupVersionKind(
		schema.GroupVersionKind{
			Group:   shared.OperatorGroup,
			Version: string(templatev1alpha1.Version),
			Kind:    string(templatev1alpha1.SampleKind),
		},
	)
	const moduleName = "test-resource"
	moduleCR.SetName(moduleName)
	moduleCR.SetNamespace(shared.DefaultRemoteNamespace)
	manifest.Spec.Resource = &moduleCR
	err = kcpClient.Create(t.Context(), manifest.Spec.Resource)
	require.NoError(t, err)

	err = kcpClient.Create(t.Context(), manifest)
	require.NoError(t, err)

	const moduleName2 = "test-resource-2"
	moduleCR2 := unstructured.Unstructured{}
	moduleCR2.SetName(moduleName2)
	moduleCR2.SetNamespace(shared.DefaultRemoteNamespace)
	moduleCR2.SetGroupVersionKind(moduleCR.GroupVersionKind())
	err = skrClient.Create(t.Context(), &moduleCR2)
	require.NoError(t, err)

	// When Getting all Module CRs
	moduleCRs, err := skrClient.GetAllModuleCRsExcludingDefaultCR(t.Context(), manifest)
	require.NoError(t, err)
	require.Len(t, moduleCRs, 2)

	// Then the non-default module CR should be returned
	assert.Equal(t, moduleCRs[0].GetName(), moduleCR.GetName())
	assert.Equal(t, moduleCRs[0].GetNamespace(), moduleCR.GetNamespace())
	assert.Equal(t, moduleCRs[1].GetName(), moduleCR2.GetName())
	assert.Equal(t, moduleCRs[1].GetNamespace(), moduleCR2.GetNamespace())
}

func getRestMapper() *meta.DefaultRESTMapper {
	sampleGVK := schema.GroupVersionKind{
		Group:   shared.OperatorGroup,
		Version: string(templatev1alpha1.Version),
		Kind:    string(templatev1alpha1.SampleKind),
	}

	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{sampleGVK.GroupVersion()})
	mapper.Add(sampleGVK, meta.RESTScopeNamespace)

	return mapper
}

func TestClient_GetAllModuleCRsExcludingDefaultCR_WithCRsInDifferentNamespaces(t *testing.T) {
	// Given a manifest CR with default CR in kyma-system namespace and additional Module CRs in different namespaces
	scheme := machineryruntime.NewScheme()
	err := v1beta2.AddToScheme(scheme)
	require.NoError(t, err)

	kcpClient := fake.NewClientBuilder().WithScheme(scheme).WithRESTMapper(getRestMapper()).Build()
	skrClient := modulecr.NewClient(kcpClient)

	manifest := testutils.NewTestManifest("test-manifest")
	moduleCR := unstructured.Unstructured{}
	moduleCR.SetGroupVersionKind(
		schema.GroupVersionKind{
			Group:   templatev1alpha1.GroupVersion.Group,
			Version: templatev1alpha1.GroupVersion.Version,
			Kind:    string(templatev1alpha1.SampleKind),
		},
	)
	const defaultModuleName = "default-resource"
	moduleCR.SetName(defaultModuleName)
	moduleCR.SetNamespace(shared.DefaultRemoteNamespace)
	manifest.Spec.Resource = &moduleCR
	err = kcpClient.Create(t.Context(), manifest.Spec.Resource)
	require.NoError(t, err)

	err = kcpClient.Create(t.Context(), manifest)
	require.NoError(t, err)

	// Create a Module CR in the default namespace
	const moduleName2 = "resource-in-default-ns"
	moduleCR2 := unstructured.Unstructured{}
	moduleCR2.SetName(moduleName2)
	moduleCR2.SetNamespace("default")
	moduleCR2.SetGroupVersionKind(moduleCR.GroupVersionKind())
	err = skrClient.Create(t.Context(), &moduleCR2)
	require.NoError(t, err)

	// Create another Module CR in a custom namespace
	const moduleName3 = "resource-in-custom-ns"
	moduleCR3 := unstructured.Unstructured{}
	moduleCR3.SetName(moduleName3)
	moduleCR3.SetNamespace("custom-namespace")
	moduleCR3.SetGroupVersionKind(moduleCR.GroupVersionKind())
	err = skrClient.Create(t.Context(), &moduleCR3)
	require.NoError(t, err)

	// Create a Module CR with the same name as default but in a different namespace
	// This tests that we compare both name AND namespace
	moduleCR4 := unstructured.Unstructured{}
	moduleCR4.SetName(defaultModuleName)
	moduleCR4.SetNamespace("another-namespace")
	moduleCR4.SetGroupVersionKind(moduleCR.GroupVersionKind())
	err = skrClient.Create(t.Context(), &moduleCR4)
	require.NoError(t, err)

	// When Getting all Module CRs excluding the default CR
	moduleCRs, err := skrClient.GetAllModuleCRsExcludingDefaultCR(t.Context(), manifest)
	require.NoError(t, err)

	// Then all non-default Module CRs should be returned (3 CRs from different namespaces)
	require.Len(t, moduleCRs, 3)

	// Verify that the default CR (same name AND namespace) is excluded
	for _, moduleCR := range moduleCRs {
		isDefaultCR := moduleCR.GetName() == defaultModuleName &&
			moduleCR.GetNamespace() == shared.DefaultRemoteNamespace
		assert.False(t, isDefaultCR, "Default Module CR should be excluded")
	}

	// Verify all expected CRs are present
	foundInDefault := false
	foundInCustom := false
	foundSameNameDiffNs := false
	for _, moduleCR := range moduleCRs {
		if moduleCR.GetName() == moduleName2 && moduleCR.GetNamespace() == "default" {
			foundInDefault = true
		}
		if moduleCR.GetName() == moduleName3 && moduleCR.GetNamespace() == "custom-namespace" {
			foundInCustom = true
		}
		if moduleCR.GetName() == defaultModuleName && moduleCR.GetNamespace() == "another-namespace" {
			foundSameNameDiffNs = true
		}
	}
	assert.True(t, foundInDefault, "Module CR in default namespace should be found")
	assert.True(t, foundInCustom, "Module CR in custom namespace should be found")
	assert.True(t, foundSameNameDiffNs, "Module CR with same name but different namespace should be found")
}

