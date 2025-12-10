package skrcontextimpl

import (
	"context"
	"errors"
	"fmt"
	"sync"

	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
)

var (
	ErrEmptyRestConfig               = errors.New("rest.Config is nil")
	ErrFailedToGetSkrClientFromCache = errors.New("failed to get SKR client from cache")
)

type Stopper interface {
	Stop() error
}

type ClientCache interface {
	Get(key client.ObjectKey) client.Client
	Add(key client.ObjectKey, value client.Client)
	Delete(key client.ObjectKey)
}

type DualClusterFactory struct {
	clientCache ClientCache
	scheme      *machineryruntime.Scheme
	event       event.Event
	SkrEnvs     sync.Map
}

func NewDualClusterFactory(scheme *machineryruntime.Scheme,
	event event.Event,
	clientCache ClientCache,
) *DualClusterFactory {
	return &DualClusterFactory{
		clientCache: clientCache,
		scheme:      scheme,
		event:       event,
		SkrEnvs:     sync.Map{},
	}
}

func (f *DualClusterFactory) Init(_ context.Context, kyma types.NamespacedName) error {
	clnt := f.clientCache.Get(kyma)
	if clnt != nil {
		// already initialized
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

	f.clientCache.Add(kyma, skrClient)

	// track this envtest so Stop() can stop all started envs
	f.SkrEnvs.Store(kyma, skrEnv)

	return err
}

func (f *DualClusterFactory) Get(kyma types.NamespacedName) (*remote.SkrContext, error) {
	client := f.clientCache.Get(kyma)
	if client == nil {
		return nil, ErrFailedToGetSkrClientFromCache
	}
	return remote.NewSkrContext(client, f.event), nil
}

func (f *DualClusterFactory) StoreEnv(name string, env *envtest.Environment) error {
	if name == "" {
		return errors.New("environment name cannot be empty")
	}
	f.SkrEnvs.Store(name, env)
	return nil
}

func (f *DualClusterFactory) InvalidateCache(_ types.NamespacedName) {
	// no-op
}

func (f *DualClusterFactory) GetSkrEnv() *envtest.Environment {
	var env *envtest.Environment
	f.SkrEnvs.Range(func(key, value any) bool {
		if e, ok := value.(*envtest.Environment); ok {
			env = e
			return false
		}
		return true
	})
	return env
}

func (f *DualClusterFactory) Stop() error {
	var errs []error

	f.SkrEnvs.Range(func(key, value any) bool {
		name, ok := key.(string)
		if !ok {
			return true
		}
		if stopper, ok := value.(Stopper); ok {
			if err := stopper.Stop(); err != nil {
				errs = append(errs, fmt.Errorf("stop %s: %w", name, err))
			}
		}
		f.SkrEnvs.Delete(key)
		return true
	})

	if len(errs) > 0 {
		return fmt.Errorf("errors stopping envtests: %w", errors.Join(errs...))
	}
	return nil
}
