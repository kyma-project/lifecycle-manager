package orphan_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/manifest/orphan"
)

const (
	kymaName              = "kyma-123"
	manifestName          = "kyma-123-template-operator-456"
	differentManifestName = "kyma-456-template-operator-789"
)

var errGeneric = errors.New("generic error")

func TestDetectionService_DetectOrphanedManifest_WhenMandatoryModule_ReturnNoError(t *testing.T) {
	service := orphan.NewDetectionService(&clientStubEmptyModuleStatus{})
	manifest := generateDefaultManifest()
	manifest.Labels[shared.IsMandatoryModule] = shared.EnableLabelValue

	err := service.DetectOrphanedManifest(t.Context(), manifest)

	require.NoError(t, err)
}

func TestDetectionService_DetectOrphanedManifest_WhenDeletionTimestampSet_ReturnNoError(t *testing.T) {
	service := orphan.NewDetectionService(&clientStubEmptyModuleStatus{})
	manifest := generateDefaultManifest()
	manifest.SetDeletionTimestamp(&apimetav1.Time{Time: time.Now()})

	err := service.DetectOrphanedManifest(t.Context(), manifest)

	require.NoError(t, err)
}

func TestDetectionService_DetectOrphanedManifest_WhenKymaLabelNotFound_ReturnError(t *testing.T) {
	service := orphan.NewDetectionService(&clientStubKymaNotFound{})
	manifest := generateDefaultManifest()
	manifest.Labels = map[string]string{}

	err := service.DetectOrphanedManifest(t.Context(), manifest)

	require.Error(t, err)
	require.ErrorIs(t, err, v1beta2.ErrLabelNotFound)
}

func TestDetectionService_DetectOrphanedManifest_WhenKymaNotFound_ReturnOrphanedManifestError(t *testing.T) {
	service := orphan.NewDetectionService(&clientStubKymaNotFound{})
	manifest := generateDefaultManifest()

	err := service.DetectOrphanedManifest(t.Context(), manifest)

	require.Error(t, err)
	require.ErrorIs(t, err, orphan.ErrOrphanedManifest)
}

func TestDetectionService_DetectOrphanedManifest_WhenClientReturnsError_ReturnError(t *testing.T) {
	service := orphan.NewDetectionService(&clientStubGenericError{})
	manifest := generateDefaultManifest()

	err := service.DetectOrphanedManifest(t.Context(), manifest)

	require.Error(t, err)
	require.NotErrorIs(t, err, orphan.ErrOrphanedManifest)
	require.ErrorIs(t, err, errGeneric)
}

func TestDetectionService_DetectOrphanedManifest_WhenEmptyModuleStatus_ReturnOrphanedManifestError(t *testing.T) {
	service := orphan.NewDetectionService(&clientStubEmptyModuleStatus{})
	manifest := generateDefaultManifest()

	err := service.DetectOrphanedManifest(t.Context(), manifest)

	require.Error(t, err)
	require.ErrorIs(t, err, orphan.ErrOrphanedManifest)
}

func TestDetectionService_DetectOrphanedManifest_WhenNoReference_ReturnOrphanedManifestError(t *testing.T) {
	service := orphan.NewDetectionService(&clientStubModuleNoReference{})
	manifest := generateDefaultManifest()

	err := service.DetectOrphanedManifest(t.Context(), manifest)

	require.Error(t, err)
	require.ErrorIs(t, err, orphan.ErrOrphanedManifest)
}

func TestDetectionService_DetectOrphanedManifest_WhenModuleStatusManifestNil_ReturnOrphanedManifestError(t *testing.T) {
	service := orphan.NewDetectionService(&clientStubModuleStatusWithNilManifest{})
	manifest := generateDefaultManifest()

	err := service.DetectOrphanedManifest(t.Context(), manifest)

	require.Error(t, err)
	require.ErrorIs(t, err, orphan.ErrOrphanedManifest)
}

func TestDetectionService_DetectOrphanedManifest_WhenValidReference_ReturnNoError(t *testing.T) {
	service := orphan.NewDetectionService(&clientStubModuleValidReference{})
	manifest := generateDefaultManifest()

	err := service.DetectOrphanedManifest(t.Context(), manifest)

	require.NoError(t, err)
}

func generateDefaultManifest() *v1beta2.Manifest {
	return &v1beta2.Manifest{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: manifestName,
			Labels: map[string]string{
				shared.KymaName: kymaName,
			},
		},
	}
}

// Client stubs

type clientStubKymaNotFound struct{}

func (c *clientStubKymaNotFound) GetKyma(_ context.Context, kymaName string, _ string) (*v1beta2.Kyma, error) {
	return nil, apierrors.NewNotFound(schema.GroupResource{Group: "v1beta2", Resource: "kyma"}, kymaName)
}

type clientStubGenericError struct{}

func (c *clientStubGenericError) GetKyma(_ context.Context, _ string, _ string) (*v1beta2.Kyma, error) {
	return nil, errGeneric
}

type clientStubEmptyModuleStatus struct{}

func (c *clientStubEmptyModuleStatus) GetKyma(_ context.Context, _ string, _ string) (*v1beta2.Kyma, error) {
	return &v1beta2.Kyma{
		Status: v1beta2.KymaStatus{
			Modules: []v1beta2.ModuleStatus{},
		},
	}, nil
}

type clientStubModuleStatusWithNilManifest struct{}

func (c *clientStubModuleStatusWithNilManifest) GetKyma(_ context.Context, _ string, _ string) (*v1beta2.Kyma, error) {
	return &v1beta2.Kyma{
		Status: v1beta2.KymaStatus{
			Modules: []v1beta2.ModuleStatus{
				{
					Manifest: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Name: differentManifestName,
						},
					},
				},
				{
					Manifest: nil,
				},
			},
		},
	}, nil
}

type clientStubModuleNoReference struct{}

func (c *clientStubModuleNoReference) GetKyma(_ context.Context, _ string, _ string) (*v1beta2.Kyma, error) {
	return &v1beta2.Kyma{
		Status: v1beta2.KymaStatus{
			Modules: []v1beta2.ModuleStatus{
				{
					Manifest: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Name: differentManifestName,
						},
					},
				},
			},
		},
	}, nil
}

type clientStubModuleValidReference struct{}

func (c *clientStubModuleValidReference) GetKyma(_ context.Context, _ string, _ string) (*v1beta2.Kyma, error) {
	return &v1beta2.Kyma{
		Status: v1beta2.KymaStatus{
			Modules: []v1beta2.ModuleStatus{
				{
					Manifest: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Name: differentManifestName,
						},
					},
				},
				{
					Manifest: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Name: manifestName,
						},
					},
				},
			},
		},
	}, nil
}
