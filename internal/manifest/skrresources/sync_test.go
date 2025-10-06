package skrresources_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/skrresources"
)

func Test_HasDiff(t *testing.T) {
	t.Parallel()
	testGVK := apimetav1.GroupVersionKind{Group: "test", Version: "v1", Kind: "test"}
	testResourceA := shared.Resource{Name: "r1", Namespace: "default", GroupVersionKind: testGVK}
	testResourceB := shared.Resource{Name: "r2", Namespace: "", GroupVersionKind: testGVK}
	testResourceC := shared.Resource{Name: "r3", Namespace: "kcp-system", GroupVersionKind: testGVK}
	testResourceD := shared.Resource{Name: "r4", Namespace: "kcp-system", GroupVersionKind: testGVK}
	tests := []struct {
		name         string
		oldResources []shared.Resource
		newResources []shared.Resource
		want         bool
	}{
		{
			"test same resource",
			[]shared.Resource{testResourceA, testResourceB},
			[]shared.Resource{testResourceA, testResourceB},
			false,
		},
		{
			"test new contains more resources",
			[]shared.Resource{testResourceA, testResourceB},
			[]shared.Resource{testResourceA, testResourceB, testResourceC},
			true,
		},
		{
			"test old contains more",
			[]shared.Resource{testResourceA, testResourceB, testResourceC},
			[]shared.Resource{testResourceA, testResourceB},
			true,
		},
		{
			"test same amount of resources but contains different name",
			[]shared.Resource{testResourceA, testResourceC},
			[]shared.Resource{testResourceA, testResourceD},
			true,
		},
		{
			"test same amount of resources but contains duplicate resources",
			[]shared.Resource{testResourceA, testResourceB},
			[]shared.Resource{testResourceA, testResourceA},
			true,
		},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assert.Equalf(t, testCase.want,
				skrresources.HasDiff(testCase.oldResources, testCase.newResources), "hasDiff(%v, %v)",
				testCase.oldResources, testCase.newResources)
		})
	}
}
