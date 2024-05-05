package skrcontextimpl

import (
	"context"

	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
)

type SingleClusterFactory struct {
	context remote.SkrContext
}

func NewSingleClusterFactory(clnt client.Client, cfg *rest.Config) *SingleClusterFactory {
	return &SingleClusterFactory{context: remote.SkrContext{Client: remote.NewClientWithConfig(clnt, cfg)}}
}

func (f *SingleClusterFactory) Init(ctx context.Context, _ types.NamespacedName) error {
	namespace := &apicorev1.Namespace{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: shared.DefaultRemoteNamespace,
			Labels: map[string]string{
				shared.ManagedBy:  shared.OperatorName,
				"istio-injection": "enabled",
				"namespaces.warden.kyma-project.io/validate": "enabled",
			},
		},
		TypeMeta: apimetav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
	}

	err := f.context.Client.Create(ctx, namespace)
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func (f *SingleClusterFactory) Get(_ types.NamespacedName) (*remote.SkrContext, error) {
	return &f.context, nil
}
