//nolint:testpackage // test private functions
package v2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/resource"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestPruneResource(t *testing.T) {
	t.Parallel()
	kubeNs := &resource.Info{
		Object: &apicorev1.Namespace{
			ObjectMeta: apimetav1.ObjectMeta{Name: "kube-system"},
			TypeMeta:   apimetav1.TypeMeta{Kind: "Namespace"},
		},
	}
	service := &resource.Info{
		Object: &apicorev1.Service{
			ObjectMeta: apimetav1.ObjectMeta{Name: "some-service"},
			TypeMeta:   apimetav1.TypeMeta{Kind: "Service"},
		},
	}
	kymaNs := &resource.Info{
		Object: &apicorev1.Namespace{
			ObjectMeta: apimetav1.ObjectMeta{Name: "kyma-system"},
			TypeMeta:   apimetav1.TypeMeta{Kind: "Namespace"},
		},
	}
	deployment := &resource.Info{
		Object: &apiappsv1.Deployment{
			ObjectMeta: apimetav1.ObjectMeta{Name: "some-deploy"},
			TypeMeta:   apimetav1.TypeMeta{Kind: "Deployment"},
		},
	}
	crd := &resource.Info{
		Object: &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: apimetav1.ObjectMeta{Name: "btpoperator"},
			TypeMeta:   apimetav1.TypeMeta{Kind: "CustomResourceDefinition"},
		},
	}

	t.Run("contains kyma-system", func(t *testing.T) {
		t.Parallel()

		infos := []*resource.Info{
			kubeNs,
			service,
			kymaNs,
			deployment,
		}

		result, err := pruneResource(infos, "Namespace", namespaceNotBeRemoved)

		require.NoError(t, err)
		require.Len(t, result, 3)
		require.NotContains(t, result, kymaNs)
	})

	t.Run("prune a crd", func(t *testing.T) {
		t.Parallel()

		infos := []*resource.Info{
			kubeNs,
			service,
			kymaNs,
			deployment,
			crd,
		}

		result, err := pruneResource(infos, "CustomResourceDefinition", "btpoperator")

		require.NoError(t, err)
		require.Len(t, result, 4)
		require.NotContains(t, result, crd)
	})

	t.Run("does not contain kyma-system", func(t *testing.T) {
		t.Parallel()

		infos := []*resource.Info{
			kubeNs,
			service,
			deployment,
		}

		result, err := pruneResource(infos, "Namespace", namespaceNotBeRemoved)

		require.NoError(t, err)
		require.Len(t, result, 3)
		require.Contains(t, result, kubeNs)
		require.Contains(t, result, service)
		require.Contains(t, result, deployment)
	})
}

func Test_hasDiff(t *testing.T) {
	t.Parallel()
	testGVK := apimetav1.GroupVersionKind{Group: "test", Version: "v1", Kind: "test"}
	testResourceA := shared.Resource{Name: "r1", Namespace: "default", GroupVersionKind: testGVK}
	testResourceB := shared.Resource{Name: "r2", Namespace: "", GroupVersionKind: testGVK}
	testResourceC := shared.Resource{Name: "r3", Namespace: "kcp-system", GroupVersionKind: testGVK}
	testResourceD := shared.Resource{Name: "r4", Namespace: "kcp-system", GroupVersionKind: testGVK}
	tests := []struct {
		name         string
		oldResources []shared.Resource
		newResources []shared.Resource
		want         bool
	}{
		{
			"test same resource",
			[]shared.Resource{testResourceA, testResourceB},
			[]shared.Resource{testResourceA, testResourceB},
			false,
		},
		{
			"test new contains more resources",
			[]shared.Resource{testResourceA, testResourceB},
			[]shared.Resource{testResourceA, testResourceB, testResourceC},
			true,
		},
		{
			"test old contains more",
			[]shared.Resource{testResourceA, testResourceB, testResourceC},
			[]shared.Resource{testResourceA, testResourceB},
			true,
		},
		{
			"test same amount of resources but contains different name",
			[]shared.Resource{testResourceA, testResourceC},
			[]shared.Resource{testResourceA, testResourceD},
			true,
		},
		{
			"test same amount of resources but contains duplicate resources",
			[]shared.Resource{testResourceA, testResourceB},
			[]shared.Resource{testResourceA, testResourceA},
			true,
		},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assert.Equalf(t, testCase.want,
				hasDiff(testCase.oldResources, testCase.newResources), "hasDiff(%v, %v)",
				testCase.oldResources, testCase.newResources)
		})
	}
}

func Test_hasStatusDiff(t *testing.T) {
	type args struct {
		first  shared.Status
		second shared.Status
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Different Status",
			args: args{
				first: shared.Status{
					State: shared.StateReady,
					LastOperation: shared.LastOperation{
						Operation:      "resources are ready",
						LastUpdateTime: apimetav1.Now(),
					},
				},
				second: shared.Status{
					State: shared.StateProcessing,
					LastOperation: shared.LastOperation{
						Operation:      "installing resources",
						LastUpdateTime: apimetav1.Now(),
					},
				},
			},
			want: true,
		},
		{
			name: "Same Status",
			args: args{
				first: shared.Status{
					State: shared.StateReady,
					LastOperation: shared.LastOperation{
						Operation:      "resources are ready",
						LastUpdateTime: apimetav1.Now(),
					},
				},
				second: shared.Status{
					State: shared.StateReady,
					LastOperation: shared.LastOperation{
						Operation:      "resources are ready",
						LastUpdateTime: apimetav1.NewTime(time.Now().Add(time.Hour)),
					},
				},
			},
			want: false,
		},
		{
			name: "Empty Status",
			args: args{
				first: shared.Status{},
				second: shared.Status{
					State: shared.StateReady,
					LastOperation: shared.LastOperation{
						Operation:      "resources are ready",
						LastUpdateTime: apimetav1.NewTime(time.Now().Add(time.Hour)),
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equalf(t, tt.want, hasStatusDiff(tt.args.first, tt.args.second), "hasStatusDiff(%v, %v)",
				tt.args.first, tt.args.second)
		})
	}
}

func Test_needsUpdateAfterLabelRemoval_WhenLabelsAreEmpty(t *testing.T) {
	emptyLabels := map[string]string{}
	res := &unstructured.Unstructured{}
	res.SetLabels(emptyLabels)
	actual := needsUpdateAfterLabelRemoval(res)

	require.Equal(t, false, actual)
	require.Equal(t, emptyLabels, res.GetLabels())
}

func Test_needsUpdateAfterLabelRemoval_WhenWatchedByLabel(t *testing.T) {
	labels := map[string]string{
		shared.WatchedByLabel: shared.WatchedByLabelValue,
		"test":                "value",
	}
	expectedLabels := map[string]string{
		"test": "value",
	}
	res := &unstructured.Unstructured{}
	res.SetLabels(labels)
	actual := needsUpdateAfterLabelRemoval(res)

	require.Equal(t, true, actual)
	require.Equal(t, expectedLabels, res.GetLabels())
}

func Test_needsUpdateAfterLabelRemoval_WhenManagedByLabel(t *testing.T) {
	labels := map[string]string{
		shared.ManagedBy: shared.ManagedByLabelValue,
	}
	expectedLabels := map[string]string{}
	res := &unstructured.Unstructured{}
	res.SetLabels(labels)
	actual := needsUpdateAfterLabelRemoval(res)

	require.Equal(t, true, actual)
	require.Equal(t, expectedLabels, res.GetLabels())
}
