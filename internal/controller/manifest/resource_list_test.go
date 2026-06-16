//nolint:testpackage // test private functions
package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

func makeResource(name, namespace, kind string) shared.Resource {
	return shared.Resource{
		Name:      name,
		Namespace: namespace,
		GroupVersionKind: apimetav1.GroupVersionKind{
			Kind: kind,
		},
	}
}

func makeObj(name, namespace, kind string) client.Object {
	res := makeResource(name, namespace, kind)
	return res.ToUnstructured()
}

func TestResourceList_Difference(t *testing.T) {
	t.Parallel()
	dummyPod := makeResource("foo", "default", "Pod")
	dummyService := makeResource("bar", "default", "Service")
	dummyDeploy := makeResource("baz", "default", "Deployment")

	list1 := ResourceList{dummyPod, dummyService, dummyDeploy}
	target := []client.Object{makeObj("bar", "default", "Service")}

	diff := list1.Difference(target)

	assert.Len(t, diff, 2)
	assert.Contains(t, diff, dummyPod)
	assert.Contains(t, diff, dummyDeploy)
	assert.NotContains(t, diff, dummyService)
}
