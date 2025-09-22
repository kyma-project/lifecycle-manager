package labelsremoval_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/labelsremoval"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func Test_RemoveManagedByLabel_WhenManifestResourcesHaveLabels(t *testing.T) {
	gvk := schema.GroupVersionKind{
		Group:   "test-group",
		Version: "v1",
		Kind:    "TestKind",
	}

	status := shared.Status{
		Synced: []shared.Resource{
			{
				Name:      "test-resource-1",
				Namespace: "test-1",
				GroupVersionKind: apimetav1.GroupVersionKind{
					Group:   gvk.Group,
					Version: gvk.Version,
					Kind:    gvk.Kind,
				},
			},
			{
				Name:      "test-resource-2",
				Namespace: "test-2",
				GroupVersionKind: apimetav1.GroupVersionKind{
					Group:   gvk.Group,
					Version: gvk.Version,
					Kind:    gvk.Kind,
				},
			},
		},
	}
	manifest := builder.NewManifestBuilder().WithStatus(status).Build()

	objs := []client.Object{
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": gvk.GroupVersion().String(),
				"kind":       gvk.Kind,
				"version":    gvk.Version,
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": gvk.GroupVersion().String(),
				"kind":       gvk.Kind,
				"version":    gvk.Version,
			},
		},
	}
	objs[0].SetName("test-resource-1")
	objs[0].SetNamespace("test-1")
	objs[0].SetLabels(map[string]string{
		"operator.kyma-project.io/managed-by": "kyma",
	})

	objs[1].SetName("test-resource-2")
	objs[1].SetNamespace("test-2")
	objs[1].SetLabels(map[string]string{
		"operator.kyma-project.io/managed-by": "kyma",
	})

	scheme := machineryruntime.NewScheme()
	err := v1beta2.AddToScheme(scheme)
	require.NoError(t, err)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	manifestClient := manifestClientStub{}

	service := labelsremoval.NewManagedByLabelRemovalService(&manifestClient)

	err = service.RemoveManagedByLabel(t.Context(), manifest, fakeClient)
	require.NoError(t, err)

	firstObj, secondObj := &unstructured.Unstructured{}, &unstructured.Unstructured{}
	firstObj.SetGroupVersionKind(gvk)
	err = fakeClient.Get(t.Context(), client.ObjectKey{Name: "test-resource-1", Namespace: "test-1"},
		firstObj)
	require.NoError(t, err)
	require.Empty(t, firstObj.GetLabels())

	secondObj.SetGroupVersionKind(gvk)
	err = fakeClient.Get(t.Context(), client.ObjectKey{Name: "test-resource-2", Namespace: "test-2"},
		secondObj)
	require.NoError(t, err)
	require.Empty(t, secondObj.GetLabels())

	assert.True(t, manifestClient.called)
}

func Test_RemoveManagedByLabel_WhenManifestResourceCannotBeFetched(t *testing.T) {
	scheme := machineryruntime.NewScheme()
	err := v1beta2.AddToScheme(scheme)
	require.NoError(t, err)

	gvk := schema.GroupVersionKind{
		Group:   "test-group",
		Version: "v1",
		Kind:    "TestKind",
	}
	status := shared.Status{
		Synced: []shared.Resource{
			{
				Name:      "test-resource-1",
				Namespace: "test-1",
				GroupVersionKind: apimetav1.GroupVersionKind{
					Group:   gvk.Group,
					Version: gvk.Version,
					Kind:    gvk.Kind,
				},
			},
		},
	}
	manifest := builder.NewManifestBuilder().WithStatus(status).Build()

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	manifestClient := manifestClientStub{}

	svc := labelsremoval.NewManagedByLabelRemovalService(&manifestClient)

	err = svc.RemoveManagedByLabel(t.Context(), manifest, fakeClient)
	require.ErrorContains(t, err, "failed to get resource")
	assert.False(t, manifestClient.called)
}

func Test_RemoveManagedByLabel_WhenDefaultCRHasLabels(t *testing.T) {
	gvk := schema.GroupVersionKind{
		Group:   "test-group",
		Version: "v1",
		Kind:    "TestKind",
	}

	defaultCR := &unstructured.Unstructured{}
	defaultCR.SetName("default-cr")
	defaultCR.SetNamespace("default-ns")
	defaultCR.SetGroupVersionKind(gvk)
	defaultCR.SetLabels(map[string]string{
		"operator.kyma-project.io/managed-by": "kyma",
	})
	objs := []client.Object{defaultCR}

	scheme := machineryruntime.NewScheme()
	err := v1beta2.AddToScheme(scheme)
	require.NoError(t, err)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	manifest := builder.NewManifestBuilder().WithResource(defaultCR).Build()
	manifestClient := manifestClientStub{}

	service := labelsremoval.NewManagedByLabelRemovalService(&manifestClient)

	err = service.RemoveManagedByLabel(t.Context(), manifest, fakeClient)

	require.NoError(t, err)

	err = fakeClient.Get(t.Context(),
		client.ObjectKey{Name: "default-cr", Namespace: "default-ns"},
		defaultCR)
	require.NoError(t, err)
	assert.Empty(t, defaultCR.GetLabels())
}

func Test_RemoveManagedByLabel_WhenDefaultCRCannotBeFetched(t *testing.T) {
	gvk := schema.GroupVersionKind{
		Group:   "test-group",
		Version: "v1",
		Kind:    "TestKind",
	}

	defaultCR := &unstructured.Unstructured{}
	defaultCR.SetName("default-cr")
	defaultCR.SetNamespace("default-ns")
	defaultCR.SetGroupVersionKind(gvk)
	defaultCR.SetLabels(map[string]string{
		"operator.kyma-project.io/managed-by": "kyma",
	})

	scheme := machineryruntime.NewScheme()
	err := v1beta2.AddToScheme(scheme)
	require.NoError(t, err)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	manifest := builder.NewManifestBuilder().WithResource(defaultCR).Build()
	manifestClient := manifestClientStub{}

	service := labelsremoval.NewManagedByLabelRemovalService(&manifestClient)

	err = service.RemoveManagedByLabel(t.Context(), manifest, fakeClient)

	require.ErrorContains(t, err, "failed to get default CR")
	assert.False(t, manifestClient.called)
}

func Test_RemoveManagedByLabel_WhenObjCannotBeUpdated(t *testing.T) {
	gvk := schema.GroupVersionKind{
		Group:   "test-group",
		Version: "v1",
		Kind:    "TestKind",
	}
	status := shared.Status{
		Synced: []shared.Resource{
			{
				Name:      "test-resource-1",
				Namespace: "test-1",
				GroupVersionKind: apimetav1.GroupVersionKind{
					Group:   gvk.Group,
					Version: gvk.Version,
					Kind:    gvk.Kind,
				},
			},
		},
	}

	objs := []client.Object{
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": gvk.GroupVersion().String(),
				"kind":       gvk.Kind,
				"version":    gvk.Version,
			},
		},
	}
	objs[0].SetName("test-resource-1")
	objs[0].SetNamespace("test-1")
	objs[0].SetLabels(map[string]string{
		"operator.kyma-project.io/managed-by": "kyma",
	})

	scheme := machineryruntime.NewScheme()
	err := v1beta2.AddToScheme(scheme)
	require.NoError(t, err)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()

	manifest := builder.NewManifestBuilder().WithStatus(status).Build()
	manifestClient := manifestClientStub{}

	service := labelsremoval.NewManagedByLabelRemovalService(&manifestClient)

	err = service.RemoveManagedByLabel(t.Context(), manifest, errorClientStub{fakeClient: fakeClient})

	require.ErrorContains(t, err, "failed to update object")
	require.ErrorContains(t, err, "test error")
}

func Test_RemoveManagedByLabel_WhenManifestResourcesAreNilAndNoDefaultCR(t *testing.T) {
	manifest := builder.NewManifestBuilder().Build()

	scheme := machineryruntime.NewScheme()
	err := v1beta2.AddToScheme(scheme)
	require.NoError(t, err)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	manifestClient := manifestClientStub{}

	service := labelsremoval.NewManagedByLabelRemovalService(&manifestClient)

	err = service.RemoveManagedByLabel(t.Context(), manifest, fakeClient)

	require.NoError(t, err)
	assert.True(t, manifestClient.called)
}

func Test_RemoveManagedByLabel_WhenFinalizerIsRemoved(t *testing.T) {
	scheme := machineryruntime.NewScheme()
	err := v1beta2.AddToScheme(scheme)
	require.NoError(t, err)

	gvk := schema.GroupVersionKind{
		Group:   "test-group",
		Version: "v1",
		Kind:    "TestKind",
	}
	status := shared.Status{
		Synced: []shared.Resource{
			{
				Name:      "test-resource-1",
				Namespace: "test-1",
				GroupVersionKind: apimetav1.GroupVersionKind{
					Group:   gvk.Group,
					Version: gvk.Version,
					Kind:    gvk.Kind,
				},
			},
		},
	}
	finalizers := []string{"label-removal-finalizer"}
	manifest := builder.NewManifestBuilder().WithFinalizers(finalizers).WithStatus(status).Build()

	objs := []client.Object{
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": gvk.GroupVersion().String(),
				"kind":       gvk.Kind,
				"version":    gvk.Version,
			},
		},
	}
	objs[0].SetName("test-resource-1")
	objs[0].SetNamespace("test-1")
	objs[0].SetLabels(map[string]string{
		"operator.kyma-project.io/managed-by": "kyma",
	})

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(manifest).WithObjects(objs...).Build()
	manifestClient := manifestClientStub{}
	svc := labelsremoval.NewManagedByLabelRemovalService(&manifestClient)

	err = svc.RemoveManagedByLabel(t.Context(), manifest, fakeClient)

	require.NoError(t, err)
	assert.Empty(t, manifest.GetFinalizers())
	assert.True(t, manifestClient.called)
}

// stubs

type manifestClientStub struct {
	called bool
	err    error
}

func (m *manifestClientStub) UpdateManifest(ctx context.Context, manifest *v1beta2.Manifest) error {
	m.called = true
	return m.err
}

type errorClientStub struct {
	client.Client

	fakeClient client.Client
}

func (e errorClientStub) Update(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
	return errors.New("test error")
}

func (e errorClientStub) Get(
	ctx context.Context,
	key client.ObjectKey,
	obj client.Object,
	opts ...client.GetOption,
) error {
	return e.fakeClient.Get(ctx, key, obj, opts...)
}
