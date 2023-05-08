//nolint:testpackage,lll
package v2

import (
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/cli-runtime/pkg/resource"
)

func TestPruneKymaSystem(t *testing.T) {
	t.Parallel()
	kubeNs := &resource.Info{Object: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}, TypeMeta: metav1.TypeMeta{Kind: "Namespace"}}}
	service := &resource.Info{Object: &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "some-service"}, TypeMeta: metav1.TypeMeta{Kind: "Service"}}}
	kymaNs := &resource.Info{Object: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kyma-system"}, TypeMeta: metav1.TypeMeta{Kind: "Namespace"}}}
	deployment := &resource.Info{Object: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "some-deploy"}, TypeMeta: metav1.TypeMeta{Kind: "Deployment"}}}

	t.Run("contains kyma-system", func(t *testing.T) {
		t.Parallel()

		infos := []*resource.Info{
			kubeNs,
			service,
			kymaNs,
			deployment,
		}

		result := pruneKymaSystem(infos)

		require.Len(t, result, 3)
		require.NotContains(t, result, kymaNs)
	})

	t.Run("does not contain kyma-system", func(t *testing.T) {
		t.Parallel()

		infos := []*resource.Info{
			kubeNs,
			service,
			deployment,
		}

		result := pruneKymaSystem(infos)

		require.Len(t, result, 3)
		require.Contains(t, result, kubeNs)
		require.Contains(t, result, service)
		require.Contains(t, result, deployment)
	})
}
