package v2_test

import (
	"context"
	"testing"

	. "github.com/kyma-project/lifecycle-manager/pkg/declarative/v2"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestConcurrentSSA(t *testing.T) {
	t.Parallel()

	_ = &resource.Info{Object: &unstructured.Unstructured{Object: map[string]any{
		"name":       "valid",
		"namespace":  "some-namespace",
		"kind":       "Pod",
		"apiVersion": "v1",
	}}}

	type args struct {
		clnt  client.Client
		owner client.FieldOwner
	}
	tests := []struct {
		name  string
		ssa   args
		apply []*resource.Info
		err   error
	}{
		{
			"simple apply nothing",
			args{
				clnt:  fake.NewClientBuilder().Build(),
				owner: client.FieldOwner("test"),
			},
			[]*resource.Info{},
			nil,
		},
		// TODO https://github.com/kubernetes/client-go/issues/970 causes SSA to not be testable
		//{
		//	"simple apply",
		//	args{
		//		clnt:  fake.NewClientBuilder().Build(),
		//		owner: client.FieldOwner("test"),
		//	},
		//	[]*resource.Info{structuredInfo},
		//	nil,
		// },
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(
			testCase.name, func(t *testing.T) {
				assertions := assert.New(t)
				t.Parallel()
				ssa := ConcurrentSSA(testCase.ssa.clnt, testCase.ssa.owner)
				if err := ssa.Run(context.Background(), testCase.apply); err != nil {
					assertions.ErrorIs(err, testCase.err)
				}
			},
		)
	}
}
