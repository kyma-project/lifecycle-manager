//nolint:testpackage,lll
package v2

import (
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/cli-runtime/pkg/resource"
)

func TestPruneResource(t *testing.T) {
	t.Parallel()
	kubeNs := &resource.Info{Object: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}, TypeMeta: metav1.TypeMeta{Kind: "Namespace"}}}
	service := &resource.Info{Object: &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "some-service"}, TypeMeta: metav1.TypeMeta{Kind: "Service"}}}
	kymaNs := &resource.Info{Object: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kyma-system"}, TypeMeta: metav1.TypeMeta{Kind: "Namespace"}}}
	deployment := &resource.Info{Object: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "some-deploy"}, TypeMeta: metav1.TypeMeta{Kind: "Deployment"}}}
	crd := &resource.Info{Object: &apiextensions.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "btpoperator"}, TypeMeta: metav1.TypeMeta{Kind: "CustomResourceDefinition"}}}

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
	testGVK := metav1.GroupVersionKind{Group: "test", Version: "v1", Kind: "test"}
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
