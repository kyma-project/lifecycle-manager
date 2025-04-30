package v2

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func TestIsManifestReferencedInKymaStatus(t *testing.T) {
	kyma := &v1beta2.Kyma{
		Status: v1beta2.KymaStatus{
			Modules: []v1beta2.ModuleStatus{
				{
					Manifest: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Name: "test-manifest-1",
						},
					},
				},
				{
					Manifest: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Name: "test-manifest-2",
						},
					},
				},
			},
		},
	}

	// Test if the function returns true for a referenced manifest
	referencedManifestName := "test-manifest-1"
	actual := isManifestReferencedInKymaStatus(kyma, referencedManifestName)
	require.True(t, actual, "Manifest %s should be referenced in Kyma status", referencedManifestName)

	orphanedManifestName := "test-manifest-6"
	actual = isManifestReferencedInKymaStatus(kyma, orphanedManifestName)
	require.False(t, actual, "Manifest %s should not be referenced in Kyma status", orphanedManifestName)
}
