package v2

import (
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	"k8s.io/cli-runtime/pkg/resource"
)

func TestPruneKymaSystem(t *testing.T) {
	obj1 := &resource.Info{Object: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}, TypeMeta: metav1.TypeMeta{Kind: "Namespace"}}}
	obj2 := &resource.Info{Object: &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "some-service"}, TypeMeta: metav1.TypeMeta{Kind: "Service"}}}
	obj3 := &resource.Info{Object: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kyma-system"}, TypeMeta: metav1.TypeMeta{Kind: "Namespace"}}}
	obj4 := &resource.Info{Object: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "some-deploy"}, TypeMeta: metav1.TypeMeta{Kind: "Deployment"}}}

	t.Run("contains kyma-system", func(t *testing.T) {
		infos := []*resource.Info{
			obj1,
			obj2,
			obj3,
			obj4,
		}

		result := pruneKymaSystem(infos)

		require.Len(t, result, 3)
		require.NotContains(t, result, obj3)
	})

	t.Run("does not contain kyma-system", func(t *testing.T) {
		infos := []*resource.Info{
			obj1,
			obj2,
			obj4,
		}

		result := pruneKymaSystem(infos)

		require.Len(t, result, 3)
		require.Contains(t, result, obj1)
		require.Contains(t, result, obj2)
		require.Contains(t, result, obj4)
	})
}
