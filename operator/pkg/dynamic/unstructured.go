package dynamic

import (
	"context"
	"github.com/kyma-project/kyma-operator/operator/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func GetUnstructuredResource(ctx context.Context, gvr schema.GroupVersionResource, name string,
	namespace string) (*unstructured.Unstructured, error) {
	c, err := config.Get()
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(c)
	if err != nil {
		return nil, err
	}

	return dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}
