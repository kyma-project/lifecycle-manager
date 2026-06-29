//nolint:testpackage // test private functions
package manifest

import (
	"testing"

	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

func makeRes(name, namespace, kind string) shared.Resource {
	return shared.Resource{
		Name:      name,
		Namespace: namespace,
		GroupVersionKind: apimetav1.GroupVersionKind{
			Kind: kind,
		},
	}
}

func TestPruneResource(t *testing.T) {
	t.Parallel()
	kubeNs := makeRes("kube-system", "", "Namespace")
	service := makeRes("some-service", "default", "Service")
	kymaNs := makeRes("kyma-system", "", "Namespace")
	deployment := makeRes("some-deploy", "default", "Deployment")
	crd := makeRes("btpoperator", "", "CustomResourceDefinition")

	t.Run("contains kyma-system", func(t *testing.T) {
		t.Parallel()

		diff := ResourceList{kubeNs, service, kymaNs, deployment}
		result := pruneResource(diff, "Namespace", shared.DefaultRemoteNamespace)

		require.Len(t, result, 3)
		require.NotContains(t, result, kymaNs)
	})

	t.Run("prune a crd", func(t *testing.T) {
		t.Parallel()

		diff := ResourceList{kubeNs, service, kymaNs, deployment, crd}
		result := pruneResource(diff, "CustomResourceDefinition", "btpoperator")

		require.Len(t, result, 4)
		require.NotContains(t, result, crd)
	})

	t.Run("does not contain kyma-system", func(t *testing.T) {
		t.Parallel()

		diff := ResourceList{kubeNs, service, deployment}
		result := pruneResource(diff, "Namespace", shared.DefaultRemoteNamespace)

		require.Len(t, result, 3)
		require.Contains(t, result, kubeNs)
		require.Contains(t, result, service)
		require.Contains(t, result, deployment)
	})
}
