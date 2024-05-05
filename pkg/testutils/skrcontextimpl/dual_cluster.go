package skrcontextimpl

import (
	"context"
	"errors"
	"sync"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	ErrEmptyRestConfig  = errors.New("rest.Config is nil")
	errSkrEnvNotStarted = errors.New("SKR envtest environment not started")
)

type DualClusterFactory struct {
	clients sync.Map
	scheme  *machineryruntime.Scheme
}

func NewDualClusterFactory(scheme *machineryruntime.Scheme) *DualClusterFactory {
	return &DualClusterFactory{
		clients: sync.Map{},
		scheme:  scheme,
	}
}

func (f *DualClusterFactory) Init(ctx context.Context, kyma types.NamespacedName) error {
	_, ok := f.clients.Load(kyma.Name)
	if ok {
		return nil
	}

	skrEnv := &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		// Scheme: scheme,
	}
	cfg, err := skrEnv.Start()
	if err != nil {
		return err
	}
	if cfg == nil {
		return ErrEmptyRestConfig
	}

	var authUser *envtest.AuthenticatedUser
	authUser, err = skrEnv.AddUser(envtest.User{
		Name:   "skr-admin-account",
		Groups: []string{"system:masters"},
	}, cfg)
	if err != nil {
		return err
	}

	skrClient, err := client.New(authUser.Config(), client.Options{Scheme: f.scheme})
	if err != nil {
		return err
	}
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
	err = skrClient.Create(ctx, namespace)
	if err != nil {
		return err
	}

	newClient := remote.NewClientWithConfig(skrClient, authUser.Config())
	f.clients.Store(kyma.Name, newClient)
	return err
}

func (f *DualClusterFactory) Get(kyma types.NamespacedName) (*remote.SkrContext, error) {
	value, ok := f.clients.Load(kyma.Name)
	if !ok {
		return nil, errSkrEnvNotStarted
	}
	skrClient, ok := value.(*remote.ConfigAndClient)
	if !ok {
		return nil, errSkrEnvNotStarted
	}
	return &remote.SkrContext{Client: skrClient}, nil
}
