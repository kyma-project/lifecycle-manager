package skrclient_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/rest"

	"github.com/kyma-project/lifecycle-manager/internal/service/manifest/skrclient"
)

// fakeMappingResolver returns a static RESTMapping for testing.
func fakeMappingResolver(_ machineryruntime.Object, _ meta.RESTMapper, _ bool) (*meta.RESTMapping, error) {
	return &meta.RESTMapping{
		Resource: schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "pods",
		},
		GroupVersionKind: schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Pod",
		},
		Scope: meta.RESTScopeNamespace,
	}, nil
}

func fakeResourceInfoClientResolver(_ *unstructured.Unstructured,
	_ *skrclient.Service,
	_ *meta.RESTMapping,
) (resource.RESTClient, error) {
	return nil, nil
}

func TestService_ResourceInfo_WithFakes(t *testing.T) {
	cfg := &rest.Config{Host: "http://example.invalid"}
	info := &skrclient.ClusterInfo{
		Config: cfg,
	}

	svc, err := skrclient.NewService(info)
	require.NoError(t, err)
	require.NotNil(t, svc)

	svc.SetMappingResolver(fakeMappingResolver)
	svc.SetResourceInfoClientResolver(fakeResourceInfoClientResolver)

	// Create a test unstructured object
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	})
	obj.SetNamespace("default")
	obj.SetName("test-pod")
	obj.SetResourceVersion("123")

	infoResult, err := svc.ResourceInfo(obj, false)
	require.NoError(t, err)
	require.NotNil(t, infoResult)
	require.Equal(t, "default", infoResult.Namespace)
	require.Equal(t, "test-pod", infoResult.Name)
	require.Equal(t, "123", infoResult.ResourceVersion)
	require.Equal(t, obj, infoResult.Object)
}
