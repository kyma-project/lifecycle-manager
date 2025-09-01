package skrclient_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/skrclient"
)

func TestUnstructuredClientForMapping_CachesAndSeparatesByGroup(t *testing.T) {
	manifest := &v1beta2.Manifest{}
	manifest.SetLabels(map[string]string{shared.KymaName: "kyma-test"})
	manifest.SetName("test-manifest")
	manifest.SetNamespace("default")

	service := skrclient.NewService(1, 1, &FakeAccessManagerService{})
	require.NotNil(t, service)

	singleton, err := service.ResolveClient(t.Context(), manifest)
	require.NoError(t, err)
	require.NotNil(t, singleton)

	coreMapping := &meta.RESTMapping{
		Resource: schema.GroupVersionResource{
			Group:    apicorev1.GroupName,
			Version:  "v1",
			Resource: "pods",
		},
		GroupVersionKind: schema.GroupVersionKind{
			Group:   apicorev1.GroupName,
			Version: "v1",
			Kind:    "Pod",
		},
		Scope: meta.RESTScopeNamespace,
	}

	clnt1, err := singleton.UnstructuredClientForMapping(coreMapping)
	require.NoError(t, err)
	require.NotNil(t, clnt1)

	clnt2, err := singleton.UnstructuredClientForMapping(coreMapping)
	require.NoError(t, err)
	require.NotNil(t, clnt2)
	require.Equal(t, clnt1, clnt2, "expected cached client to be returned on second call, but got a different instance")

	otherMapping := &meta.RESTMapping{
		Resource: schema.GroupVersionResource{
			Group:    "mygroup.example.com",
			Version:  "v1alpha1",
			Resource: "mykinds",
		},
		GroupVersionKind: schema.GroupVersionKind{
			Group:   "mygroup.example.com",
			Version: "v1alpha1",
			Kind:    "MyKind",
		},
		Scope: meta.RESTScopeRoot,
	}

	clnt3, err := singleton.UnstructuredClientForMapping(otherMapping)
	require.NoError(t, err)
	require.NotNil(t, clnt3)
	require.NotEqual(t, clnt1, clnt3, "expected different client instances for different mapping groups")
}
