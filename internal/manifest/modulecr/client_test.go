package modulecr_test

import (
	"testing"

	templatev1alpha1 "github.com/kyma-project/template-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/scheme"

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
	err = addSampleToScheme(testScheme)
	require.NoError(t, err)

	kcpClient := fake.NewClientBuilder().WithScheme(testScheme).Build()
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

func TestClient_GetAllModuleCRsExcludingDefaultCR_WithIgnorePolicy(t *testing.T) {
	// Given a manifest CR and two resource CRs deployed in the cluster
	testScheme := machineryruntime.NewScheme()
	err := v1beta2.AddToScheme(testScheme)
	require.NoError(t, err)
	err = addSampleToScheme(testScheme)
	require.NoError(t, err)

	kcpClient := fake.NewClientBuilder().WithScheme(testScheme).Build()
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

func addSampleToScheme(testScheme *machineryruntime.Scheme) error {
	groupVersion := schema.GroupVersion{
		Group:   shared.OperatorGroup,
		Version: string(templatev1alpha1.Version),
	}
	schemeBuilder := &scheme.Builder{GroupVersion: groupVersion}
	return schemeBuilder.AddToScheme(testScheme)
}
