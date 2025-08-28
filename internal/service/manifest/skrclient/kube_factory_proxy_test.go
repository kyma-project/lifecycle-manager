package skrclient_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/service/manifest/skrclient"
	apicorev1 "k8s.io/api/core/v1"
	metapkg "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

func TestUnstructuredClientForMapping_CachesAndSeparatesByGroup(t *testing.T) {
	cfg := &rest.Config{Host: "http://example.invalid"}
	info := &skrclient.ClusterInfo{
		Config: cfg,
	}

	svc, err := skrclient.NewService(info)
	require.NoError(t, err)
	require.NotNil(t, svc)

	coreMapping := &metapkg.RESTMapping{
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
		Scope: metapkg.RESTScopeNamespace,
	}

	c1, err := svc.UnstructuredClientForMapping(coreMapping)
	require.NoError(t, err)
	require.NotNil(t, c1)

	c2, err := svc.UnstructuredClientForMapping(coreMapping)
	require.NoError(t, err)
	require.NotNil(t, c2)
	require.Equal(t, c1, c2, "expected cached client to be returned on second call, but got a different instance")

	otherMapping := &metapkg.RESTMapping{
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
		Scope: metapkg.RESTScopeRoot,
	}

	c3, err := svc.UnstructuredClientForMapping(otherMapping)
	require.NoError(t, err)
	require.NotNil(t, c3)
	require.NotEqual(t, c1, c3, "expected different client instances for different mapping groups")
}
