package skrcontextimpl

import (
	"context"
	"errors"
	"sync"

	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"fmt"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
)

var (
	ErrEmptyRestConfig  = errors.New("rest.Config is nil")
	ErrSkrEnvNotStarted = errors.New("SKR envtest environment not started")
)

type DualClusterFactory struct {
	clients sync.Map
	scheme  *machineryruntime.Scheme
	event   event.Event
	skrEnv  *envtest.Environment
	skrEnvs sync.Map
}

func NewDualClusterFactory(scheme *machineryruntime.Scheme, event event.Event) *DualClusterFactory {
	return &DualClusterFactory{
		clients: sync.Map{},
		scheme:  scheme,
		event:   event,
		skrEnvs: sync.Map{},
	}
}

func (f *DualClusterFactory) Init(_ context.Context, kyma types.NamespacedName) error {
	_, ok := f.clients.Load(kyma.Name)
	if ok {
		return nil
	}

	skrEnv := &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		// Scheme: scheme,
	}

	// Start the envtest and record the returned cfg
	cfg, err := skrEnv.Start()
	if err != nil {
		return err
	}
	if cfg == nil {
		// cleanup fast - if start returned nil cfg
		_ = skrEnv.Stop()
		return ErrEmptyRestConfig
	}

	var authUser *envtest.AuthenticatedUser
	authUser, err = skrEnv.AddUser(envtest.User{
		Name:   "skr-admin-account",
		Groups: []string{"system:masters"},
	}, cfg)
	if err != nil {
		_ = skrEnv.Stop()
		return err
	}

	skrClient, err := client.New(authUser.Config(), client.Options{Scheme: f.scheme})
	if err != nil {
		_ = skrEnv.Stop()
		return err
	}
	newClient := remote.NewClientWithConfig(skrClient, authUser.Config())
	f.clients.Store(kyma.Name, newClient)

	f.skrEnv = skrEnv
	// track this envtest so Stop() can stop all started envs
	f.skrEnvs.Store(kyma.Name, skrEnv)

	return err
}

func (f *DualClusterFactory) Get(kyma types.NamespacedName) (*remote.SkrContext, error) {
	value, ok := f.clients.Load(kyma.Name)
	if !ok {
		return nil, ErrSkrEnvNotStarted
	}
	skrClient, ok := value.(*remote.ConfigAndClient)
	if !ok {
		return nil, ErrSkrEnvNotStarted
	}
	return remote.NewSkrContext(skrClient, f.event), nil
}

func (f *DualClusterFactory) StoreEnv(name string, env interface{}) {
	f.skrEnvs.Store(name, env)
}

func (f *DualClusterFactory) InvalidateCache(_ types.NamespacedName) {
	// no-op
}

func (f *DualClusterFactory) GetSkrEnv() *envtest.Environment {
	return f.skrEnv
}

func (f *DualClusterFactory) Stop() error {
	var errs []error

	f.skrEnvs.Range(func(key, value interface{}) bool {
		name := ""
		if ks, ok := key.(string); ok {
			name = ks
		}

		if env, ok := value.(*envtest.Environment); ok && env != nil {
			if err := env.Stop(); err != nil {
				if name != "" {
					errs = append(errs, fmt.Errorf("stop envtest %q: %w", name, err))
				} else {
					errs = append(errs, fmt.Errorf("stop envtest (unknown key): %w", err))
				}
			}
		}

		// remove entries so we don't double-stop later
		f.skrEnvs.Delete(key)
		if name != "" {
			f.clients.Delete(name)
		}
		return true
	})

	// Clear skrEnv
	f.skrEnv = nil

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("errors stopping envtests: %w", errors.Join(errs...))
}
