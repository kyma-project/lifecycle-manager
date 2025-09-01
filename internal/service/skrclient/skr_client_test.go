package skrclient_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/rest"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/skrclient"
)

type FakeAccessManagerService struct{}

func (f *FakeAccessManagerService) GetAccessRestConfigByKyma(_ context.Context, _ string) (*rest.Config, error) {
	return &rest.Config{Host: "http://example.invalid"}, nil
}

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
	_ *skrclient.SingletonClient,
	_ *meta.RESTMapping,
) (resource.RESTClient, error) {
	return nil, nil
}

func TestSingletonClient_ResourceInfo_WithClientResolver(t *testing.T) {
	manifest := &v1beta2.Manifest{}
	manifest.SetLabels(map[string]string{shared.KymaName: "kyma-test"})
	manifest.SetName("test-manifest")
	manifest.SetNamespace("default")

	service := skrclient.NewService(1, 1, &FakeAccessManagerService{})
	singleton, err := service.ResolveClient(t.Context(), manifest)
	require.NoError(t, err)
	require.NotNil(t, singleton)

	singleton.SetMappingResolver(fakeMappingResolver)
	singleton.SetResourceInfoClientResolver(fakeResourceInfoClientResolver)

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"})
	obj.SetNamespace("default")
	obj.SetName("test-pod")
	obj.SetResourceVersion("123")

	infoResult, err := singleton.ResourceInfo(obj, false)
	require.NoError(t, err)
	require.NotNil(t, infoResult)
	require.Equal(t, "default", infoResult.Namespace)
	require.Equal(t, "test-pod", infoResult.Name)
	require.Equal(t, "123", infoResult.ResourceVersion)
	require.Equal(t, obj, infoResult.Object)
}
