package skrresources_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/skrresources"
)

func TestConcurrentSSA(t *testing.T) {
	t.Parallel()

	pod := &unstructured.Unstructured{
		Object: map[string]any{
			"kind":       "Pod",
			"apiVersion": "v1",
			"metadata": map[string]any{
				"name":      "valid",
				"namespace": "some-namespace",
			},
		},
	}
	fakeClientBuilder := fake.NewClientBuilder().WithRuntimeObjects(pod).Build()
	_ = fakeClientBuilder.Create(t.Context(), pod)

	inactiveCollector := skrresources.NewManifestLogCollector(nil, fieldowners.DeclarativeApplier)

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
				owner: fieldowners.LifecycleManager,
			},
			[]*resource.Info{},
			nil,
		},
	}

	for _, testCase := range tests {
		t.Run(
			testCase.name, func(t *testing.T) {
				t.Parallel()
				ssa := skrresources.ConcurrentSSA(testCase.ssa.clnt, testCase.ssa.owner, inactiveCollector)
				if err := ssa.Run(t.Context(), testCase.apply); err != nil {
					require.ErrorIs(t, err, testCase.err)
				}
			},
		)
	}
}
