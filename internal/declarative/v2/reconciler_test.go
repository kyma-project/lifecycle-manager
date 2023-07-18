//nolint:testpackage,lll
package v2

import (
	"testing"

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

		result := pruneResource(infos, "Namespace", namespaceNotBeRemoved)

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

		result := pruneResource(infos, "CustomResourceDefinition", "btpoperator")

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

		result := pruneResource(infos, "Namespace", namespaceNotBeRemoved)

		require.Len(t, result, 3)
		require.Contains(t, result, kubeNs)
		require.Contains(t, result, service)
		require.Contains(t, result, deployment)
	})
}

func Test_hasDiff(t *testing.T) {
	testGVK := metav1.GroupVersionKind{Group: "test", Version: "v1", Kind: "test"}
	testResourceA := Resource{Name: "r1", Namespace: "default", GroupVersionKind: testGVK}
	testResourceB := Resource{Name: "r2", Namespace: "", GroupVersionKind: testGVK}
	testResourceC := Resource{Name: "r3", Namespace: "kcp-system", GroupVersionKind: testGVK}
	testResourceD := Resource{Name: "r4", Namespace: "kcp-system", GroupVersionKind: testGVK}
	tests := []struct {
		name         string
		oldResources []Resource
		newResources []Resource
		want         bool
	}{
		{
			"test same resource",
			[]Resource{testResourceA, testResourceB},
			[]Resource{testResourceA, testResourceB},
			false,
		},
		{
			"test new contains more resources",
			[]Resource{testResourceA, testResourceB},
			[]Resource{testResourceA, testResourceB, testResourceC},
			true,
		},
		{
			"test old contains more",
			[]Resource{testResourceA, testResourceB, testResourceC},
			[]Resource{testResourceA, testResourceB},
			true,
		},
		{
			"test same amount of resources but contains different name",
			[]Resource{testResourceA, testResourceC},
			[]Resource{testResourceA, testResourceD},
			true,
		},
		{
			"test same amount of resources but contains duplicate resources",
			[]Resource{testResourceA, testResourceB},
			[]Resource{testResourceA, testResourceA},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, hasDiff(tt.oldResources, tt.newResources), "hasDiff(%v, %v)", tt.oldResources, tt.newResources)
		})
	}
}
