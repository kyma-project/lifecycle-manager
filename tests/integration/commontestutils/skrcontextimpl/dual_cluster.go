package skrcontextimpl

import (
	"context"
	"errors"
	"sync"

	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
)

var (
	ErrEmptyRestConfig  = errors.New("rest.Config is nil")
	errSkrEnvNotStarted = errors.New("SKR envtest environment not started")
)

// DualClusterFactory starts one shared envtest environment and reuses it for every Kyma name.
// Bug fix: previously a new environment was started per Kyma and the old one wasn't stopped,
// leaking api-server/etcd processes. Now we only create it once and keep the returned rest.Config.
type DualClusterFactory struct {
    clients     sync.Map // kymaName -> *remote.ConfigAndClient
    scheme      *machineryruntime.Scheme
    event       event.Event
    skrEnv      *envtest.Environment
    restConfig  *rest.Config
}

func NewDualClusterFactory(scheme *machineryruntime.Scheme, event event.Event) *DualClusterFactory {
	return &DualClusterFactory{
		clients: sync.Map{},
		scheme:  scheme,
		event:   event,
	}
}

func (f *DualClusterFactory) Init(_ context.Context, kyma types.NamespacedName) error {
    if _, ok := f.clients.Load(kyma.Name); ok {
        return nil
    }
    // Start environment only once.
    if f.skrEnv == nil {
        f.skrEnv = &envtest.Environment{ErrorIfCRDPathMissing: true}
        cfg, err := f.skrEnv.Start()
        if err != nil {
            return err
        }
        if cfg == nil {
            return ErrEmptyRestConfig
        }
        f.restConfig = cfg
    }
    // For each new Kyma we create a new admin user & client (cheap) against the same env.
    authUser, err := f.skrEnv.AddUser(envtest.User{
        Name:   "skr-admin-account",
        Groups: []string{"system:masters"},
    }, f.restConfig)
    if err != nil {
        return err
    }
    skrClient, err := client.New(authUser.Config(), client.Options{Scheme: f.scheme})
    if err != nil {
        return err
    }
    f.clients.Store(kyma.Name, remote.NewClientWithConfig(skrClient, authUser.Config()))
    return nil
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
	return remote.NewSkrContext(skrClient, f.event), nil
}

func (f *DualClusterFactory) InvalidateCache(_ types.NamespacedName) {
	// no-op
}

func (f *DualClusterFactory) GetSkrEnv() *envtest.Environment {
	return f.skrEnv
}

func (f *DualClusterFactory) Stop() error {
    if f.skrEnv == nil {
        return nil
    }
    err := f.skrEnv.Stop()
    f.skrEnv = nil
    f.restConfig = nil
    return err
}
