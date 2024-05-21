package skrcontextimpl

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/remote"
)

type SingleClusterFactory struct {
	context remote.SkrContext
}

func NewSingleClusterFactory(clnt client.Client, cfg *rest.Config) *SingleClusterFactory {
	return &SingleClusterFactory{context: remote.SkrContext{Client: remote.NewClientWithConfig(clnt, cfg)}}
}

func (f *SingleClusterFactory) Init(_ context.Context, _ types.NamespacedName) error {
	return nil
}

func (f *SingleClusterFactory) Get(_ types.NamespacedName) (*remote.SkrContext, error) {
	return &f.context, nil
}

func (f *SingleClusterFactory) InvalidateCache(_ types.NamespacedName) {
	// no-op
}
