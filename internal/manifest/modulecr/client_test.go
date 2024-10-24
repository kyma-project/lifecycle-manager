package modulecr_test

import (
	"context"
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

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/finalizer"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/modulecr"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

func TestClient_RemoveModuleCR(t *testing.T) {
	// Given a manifest CR with finalizer and a resource CR deployed in the cluster
	scheme := machineryruntime.NewScheme()
	err := v1beta2.AddToScheme(scheme)
	require.NoError(t, err)

	kcpClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	skrClient := modulecr.NewClient(kcpClient)
	ctx := context.TODO()

	manifest := testutils.NewTestManifest("test-manifest")
	manifest.SetFinalizers([]string{modulecr.CustomResourceManagerFinalizer, finalizer.DefaultFinalizer})
	moduleCR := unstructured.Unstructured{}
	moduleCR.SetGroupVersionKind(
		schema.GroupVersionKind{
			Group:   templatev1alpha1.GroupVersion.Group,
			Version: templatev1alpha1.GroupVersion.Version,
			Kind:    string(templatev1alpha1.SampleKind),
		},
	)
	moduleCR.SetName("test-resource")
	moduleCR.SetNamespace(shared.DefaultRemoteNamespace)
	manifest.Spec.Resource = &moduleCR
	err = kcpClient.Create(ctx, manifest.Spec.Resource)
	require.NoError(t, err)

	err = kcpClient.Create(ctx, manifest)
	require.NoError(t, err)

	// When Manifest CR is under deletion, fakeClient does not support setting deletionTimestamp
	deletionTimestamp := apimetav1.Now()
	manifest.SetDeletionTimestamp(&deletionTimestamp)

	// And deleting the resource CR
	err = skrClient.RemoveModuleCR(ctx, kcpClient, manifest)
	require.NoError(t, err)

	// And in second deletion attempt, the resource should not be found
	err = skrClient.RemoveModuleCR(ctx, kcpClient, manifest)
	require.ErrorIs(t, err, modulecr.ErrRequeueRequired)

	// Then the resource CR should be deleted and the finalizer should be removed from Manifest CR
	err = kcpClient.Get(ctx, client.ObjectKey{Name: moduleCR.GetName(), Namespace: moduleCR.GetNamespace()},
		&moduleCR)
	require.True(t, apierrors.IsNotFound(err))

	err = kcpClient.Get(ctx, client.ObjectKey{Name: manifest.GetName(), Namespace: manifest.GetNamespace()}, manifest)
	require.NoError(t, err)
	assert.NotContains(t, manifest.GetFinalizers(), modulecr.CustomResourceManagerFinalizer)
}
