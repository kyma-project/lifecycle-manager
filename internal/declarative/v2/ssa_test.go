package v2_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
)

func TestConcurrentSSA(t *testing.T) {
	t.Parallel()

	pod := &unstructured.Unstructured{Object: map[string]any{
		"kind":       "Pod",
		"apiVersion": "v1",
		"metadata": map[string]interface{}{
			"name":      "valid",
			"namespace": "some-namespace",
		},
	}}
	structuredInfo := &resource.Info{Object: pod}
	fakeClientBuilder := fake.NewClientBuilder().WithRuntimeObjects(pod).Build()
	_ = fakeClientBuilder.Create(context.Background(), pod)

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
				clnt:  fakeClientBuilder,
				owner: client.FieldOwner("test"),
			},
			[]*resource.Info{},
			nil,
		},
		{
			"simple apply",
			args{
				clnt:  fakeClientBuilder,
				owner: client.FieldOwner("test"),
			},
			[]*resource.Info{structuredInfo},
			nil,
		},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(
			testCase.name, func(t *testing.T) {
				t.Parallel()
				ssa := declarative.ConcurrentSSA(testCase.ssa.clnt, testCase.ssa.owner)
				if err := ssa.Run(context.Background(), testCase.apply); err != nil {
					require.ErrorIs(t, err, testCase.err)
				}
			},
		)
	}
}
