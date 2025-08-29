package skrclient_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"

	"github.com/kyma-project/lifecycle-manager/internal/service/manifest/skrclient"
)

func TestUnstructuredClientForMapping_CachesAndSeparatesByGroup(t *testing.T) {
	cfg := &rest.Config{Host: "http://example.invalid"}
	info := &skrclient.ClusterInfo{
		Config: cfg,
	}

	svc, err := skrclient.NewService(info)
	require.NoError(t, err)
	require.NotNil(t, svc)

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

	clnt1, err := svc.UnstructuredClientForMapping(coreMapping)
	require.NoError(t, err)
	require.NotNil(t, clnt1)

	clnt2, err := svc.UnstructuredClientForMapping(coreMapping)
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

	clnt3, err := svc.UnstructuredClientForMapping(otherMapping)
	require.NoError(t, err)
	require.NotNil(t, clnt3)
	require.NotEqual(t, clnt1, clnt3, "expected different client instances for different mapping groups")
}
