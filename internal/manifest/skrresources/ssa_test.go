package skrresources_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
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
		apply []client.Object
		err   error
	}{
		{
			"simple apply nothing",
			args{
				clnt:  fakeClientBuilder,
				owner: fieldowners.LifecycleManager,
			},
			[]client.Object{},
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

// TestConcurrentSSA_ErrorStringIsDeterministic verifies that when multiple resources
// fail concurrently the combined error string is identical across runs. Without sorting,
// goroutine scheduling produces non-deterministic errors.Join output which defeats
// HasStatusDiff-based change detection and bypasses exponential backoff.
func TestConcurrentSSA_ErrorStringIsDeterministic(t *testing.T) {
	t.Parallel()

	resources := []client.Object{
		&unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1", "kind": "ConfigMap",
			"metadata": map[string]any{"name": "z-resource", "namespace": "test"},
		}},
		&unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1", "kind": "ConfigMap",
			"metadata": map[string]any{"name": "a-resource", "namespace": "test"},
		}},
	}

	mc := &alwaysErrClient{Client: fake.NewClientBuilder().Build(), err: errors.New("injected failure")}
	collector := skrresources.NewManifestLogCollector(nil, fieldowners.DeclarativeApplier)
	ssa := skrresources.ConcurrentSSA(mc, fieldowners.LifecycleManager, collector)

	var first string
	for range 20 {
		err := ssa.Run(t.Context(), resources)
		require.Error(t, err)
		if first == "" {
			first = err.Error()
		}
		require.Equal(t, first, err.Error(), "error string must be deterministic across runs")
	}
}

type alwaysErrClient struct {
	client.Client

	err error
}

func (a *alwaysErrClient) Apply(
	_ context.Context, _ machineryruntime.ApplyConfiguration, _ ...client.ApplyOption,
) error {
	return a.err
}
