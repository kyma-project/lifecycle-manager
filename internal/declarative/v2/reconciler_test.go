//nolint:testpackage // test private functions
package v2

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/resource"
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
