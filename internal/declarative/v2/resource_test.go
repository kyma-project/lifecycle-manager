package v2_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"

	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
)

func newInfo(name, namespace, kind string) *resource.Info {
	return &resource.Info{
		Name:      name,
		Namespace: namespace,
		Mapping: &meta.RESTMapping{
			GroupVersionKind: schema.GroupVersionKind{Kind: kind},
		},
	}
}

func TestResourceList_Difference(t *testing.T) {
	dummyPod := newInfo("foo", "default", "Pod")
	dummyService := newInfo("bar", "default", "Service")
	dummyDeploy := newInfo("baz", "default", "Deployment")

	list1 := declarativev2.ResourceList{dummyPod, dummyService, dummyDeploy}
	list2 := declarativev2.ResourceList{dummyService}

	diff := list1.Difference(list2)

	assert.Len(t, diff, 2)
	assert.Contains(t, diff, dummyPod)
	assert.Contains(t, diff, dummyDeploy)
	assert.NotContains(t, diff, dummyService)
}
