package finalizer_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	k8sclientscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/finalizer"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

func TestRemoveCRFinalizer(t *testing.T) {
	// Given a manifest with a finalizer
	ctx := context.TODO()
	scheme := k8sclientscheme.Scheme
	require.NoError(t, v1beta2.AddToScheme(scheme))

	manifest := testutils.NewTestManifest("test-manifest")
	manifest.SetFinalizers([]string{finalizer.CustomResourceManagerFinalizer})
	manifest.Spec.Resource = testutils.NewTestModuleCR(shared.DefaultRemoteNamespace)
	kcpClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(manifest).Build()

	// When RemoveCRFinalizer is called
	err := finalizer.RemoveCRFinalizer(ctx, kcpClient, manifest)

	// Then the finalizer should be removed and ErrRequeueRequired should be returned
	require.ErrorIs(t, err, finalizer.ErrRequeueRequired)

	updatedManifest := &v1beta2.Manifest{}
	err = kcpClient.Get(ctx, types.NamespacedName{Name: manifest.Name, Namespace: manifest.Namespace}, updatedManifest)
	require.NoError(t, err)
	assert.NotContains(t, updatedManifest.GetFinalizers(), finalizer.CustomResourceManagerFinalizer)

	// When RemoveCRFinalizer is called again
	err = finalizer.RemoveCRFinalizer(ctx, kcpClient, manifest)

	// Then it should return nil as the finalizer is already removed
	assert.NoError(t, err)
}
