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
	"github.com/kyma-project/lifecycle-manager/internal/service/accessmanager"
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
	clientCache  ClientCache
	scheme       *machineryruntime.Scheme
	event        event.Event
	SkrEnvs      sync.Map
	stoppedKymas sync.Map
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
	// Once StopEnvForKyma has been called for a Kyma, return ErrAccessSecretNotFound
	// so the controller routes deleting Kymas straight to processDeletion, which skips
	// all SKR-facing steps when the client is no longer in cache.
	if _, stopped := f.stoppedKymas.Load(kyma.Name); stopped {
		return accessmanager.ErrAccessSecretNotFound
	}

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
	f.SkrEnvs.Store(kyma.Name, skrEnv)

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

// StopEnvForKyma stops and removes the SKR envtest environment for the given Kyma.
// It marks the Kyma as stopped first so that Init() returns ErrAccessSecretNotFound
// for any future reconcile, routing deleting Kymas through processDeletion (which
// skips all SKR steps when the client is absent from cache) so deletion completes quickly.
func (f *DualClusterFactory) StopEnvForKyma(kymaName types.NamespacedName) error {
	f.stoppedKymas.Store(kymaName.Name, struct{}{})

	val, loaded := f.SkrEnvs.LoadAndDelete(kymaName.Name)
	if !loaded {
		return nil
	}
	f.clientCache.Delete(kymaName)
	if stopper, ok := val.(Stopper); ok {
		return stopper.Stop()
	}
	return nil
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
