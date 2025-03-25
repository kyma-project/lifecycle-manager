package skrcontextimpl

import (
	"context"
	"errors"
	"sync"

	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
)

var (
	ErrEmptyRestConfig  = errors.New("rest.Config is nil")
	errSkrEnvNotStarted = errors.New("SKR envtest environment not started")
)

type DualClusterFactory struct {
	clients sync.Map
	scheme  *machineryruntime.Scheme
	event   event.Event
	skrEnv  *envtest.Environment
}

func NewDualClusterFactory(scheme *machineryruntime.Scheme, event event.Event) *DualClusterFactory {
	return &DualClusterFactory{
		clients: sync.Map{},
		scheme:  scheme,
		event:   event,
	}
}

func (f *DualClusterFactory) Init(_ context.Context, kyma types.NamespacedName) error {
	_, ok := f.clients.Load(kyma.Name)
	if ok {
		return nil
	}

	f.skrEnv = &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		// Scheme: scheme,
	}
	cfg, err := f.GetSkrEnv().Start()
	if err != nil {
		return err
	}
	if cfg == nil {
		return ErrEmptyRestConfig
	}

	var authUser *envtest.AuthenticatedUser
	authUser, err = f.GetSkrEnv().AddUser(envtest.User{
		Name:   "skr-admin-account",
		Groups: []string{"system:masters"},
	}, cfg)
	if err != nil {
		return err
	}

	skrClient, err := client.New(authUser.Config(), client.Options{Scheme: f.scheme})
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
	return f.skrEnv.Stop()
}
