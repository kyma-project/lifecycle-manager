package labelsremoval_test

import (
	"context"
	"testing"

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
	"github.com/kyma-project/lifecycle-manager/internal/manifest/manifestclient"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func Test_needsUpdateAfterLabelRemoval_WhenLabelsAreEmpty(t *testing.T) {
	emptyLabels := map[string]string{}
	res := &unstructured.Unstructured{}
	res.SetLabels(emptyLabels)
	actual := labelsremoval.IsManagedLabelRemoved(res)

	require.False(t, actual)
	require.Equal(t, emptyLabels, res.GetLabels())
}

func Test_needsUpdateAfterLabelRemoval_WhenManagedByLabel(t *testing.T) {
	labels := map[string]string{
		shared.ManagedBy: shared.ManagedByLabelValue,
	}
	expectedLabels := map[string]string{}
	res := &unstructured.Unstructured{}
	res.SetLabels(labels)
	actual := labelsremoval.IsManagedLabelRemoved(res)

	require.True(t, actual)
	require.Equal(t, expectedLabels, res.GetLabels())
}

func Test_handleLabelsRemovalFromResources_WhenManifestResourcesHaveLabels(t *testing.T) {
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

	err = labelsremoval.HandleLabelsRemovalFromResources(context.TODO(), manifest, fakeClient, nil)
	require.NoError(t, err)

	firstObj, secondObj := &unstructured.Unstructured{}, &unstructured.Unstructured{}
	firstObj.SetGroupVersionKind(gvk)
	err = fakeClient.Get(context.TODO(), client.ObjectKey{Name: "test-resource-1", Namespace: "test-1"},
		firstObj)
	require.NoError(t, err)
	require.Empty(t, firstObj.GetLabels())

	secondObj.SetGroupVersionKind(gvk)
	err = fakeClient.Get(context.TODO(), client.ObjectKey{Name: "test-resource-2", Namespace: "test-2"},
		secondObj)
	require.NoError(t, err)
	require.Empty(t, secondObj.GetLabels())
}

func Test_handleLabelsRemovalFromResources_WhenManifestResourcesAreNilAndNoDefaultCR(t *testing.T) {
	manifest := builder.NewManifestBuilder().Build()

	scheme := machineryruntime.NewScheme()
	err := v1beta2.AddToScheme(scheme)
	require.NoError(t, err)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	err = labelsremoval.HandleLabelsRemovalFromResources(context.TODO(), manifest, fakeClient, nil)

	require.NoError(t, err)
}

func Test_handleLabelsRemovalFromResources_WhenManifestResourcesAndDefaultCRHaveLabels(t *testing.T) {
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

	defaultCR := &unstructured.Unstructured{}
	defaultCR.SetName("default-cr")
	defaultCR.SetNamespace("default-ns")
	defaultCR.SetGroupVersionKind(gvk)
	defaultCR.SetLabels(map[string]string{
		"operator.kyma-project.io/managed-by": "kyma",
	})

	objs = append(objs, defaultCR)

	scheme := machineryruntime.NewScheme()
	err := v1beta2.AddToScheme(scheme)
	require.NoError(t, err)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()

	err = labelsremoval.HandleLabelsRemovalFromResources(context.TODO(), manifest, fakeClient,
		defaultCR)

	require.NoError(t, err)

	firstObj := &unstructured.Unstructured{}
	firstObj.SetGroupVersionKind(gvk)
	err = fakeClient.Get(context.TODO(), client.ObjectKey{Name: "test-resource-1", Namespace: "test-1"},
		firstObj)
	require.NoError(t, err)
	require.Empty(t, firstObj.GetLabels())

	require.NoError(t, err)
	require.Empty(t, defaultCR.GetLabels())
}

func Test_HandleLabelsRemovalFinalizerForUnmanagedModule_WhenErrorIsReturned(t *testing.T) {
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

	manifestClnt := manifestclient.NewManifestClient(nil, fakeClient)
	svc := labelsremoval.NewManagedLabelRemovalService(manifestClnt)

	err = svc.HandleLabelsRemovalFinalizerForUnmanagedModule(context.TODO(), manifest, fakeClient, nil)
	require.ErrorContains(t, err, "failed to get resource")
}

func Test_HandleLabelsRemovalFinalizerForUnmanagedModule_WhenFinalizerIsRemoved(t *testing.T) {
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

	manifestClnt := manifestclient.NewManifestClient(nil, fakeClient)
	svc := labelsremoval.NewManagedLabelRemovalService(manifestClnt)

	err = svc.HandleLabelsRemovalFinalizerForUnmanagedModule(context.TODO(), manifest, fakeClient, nil)
	require.NoError(t, err)

	err = fakeClient.Get(context.TODO(), client.ObjectKey{Name: manifest.GetName(), Namespace: manifest.GetNamespace()},
		manifest)
	require.NoError(t, err)
	require.Empty(t, manifest.GetFinalizers())
}
