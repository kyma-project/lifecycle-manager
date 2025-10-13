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

// DualClusterFactory provides a single shared SKR (remote) envtest environment for
// all Kyma instances created during a suite run. Previously a new envtest.Environment
// was started for every previously unseen Kyma name, replacing the pointer to the
// old environment without stopping it. That leaked kube-apiserver / etcd processes
// which accumulated across test runs. Now we lazily create exactly one environment
// and reuse its client for each Kyma. Additional Kyma names simply reuse the same
// underlying remote cluster. The factory is safe for concurrent use.
type DualClusterFactory struct {
	clients   sync.Map // map[string]*remote.ConfigAndClient keyed by kyma name
	scheme    *machineryruntime.Scheme
	event     event.Event
	skrEnv    *envtest.Environment
	baseOnce  sync.Once                // ensure environment started exactly once
	baseErr   error                    // capture start error
	baseUser  *envtest.AuthenticatedUser
	baseClient *remote.ConfigAndClient // cached client used for subsequent kyma names
	mu        sync.Mutex               // protects Stop from racing with Init
}

func NewDualClusterFactory(scheme *machineryruntime.Scheme, event event.Event) *DualClusterFactory {
	return &DualClusterFactory{
		clients: sync.Map{},
		scheme:  scheme,
		event:   event,
	}
}

func (f *DualClusterFactory) Init(_ context.Context, kyma types.NamespacedName) error {
	// Fast path: already have client for this kyma.
	if _, ok := f.clients.Load(kyma.Name); ok {
		return nil
	}

	// Lazily start shared environment exactly once.
	f.baseOnce.Do(func() {
		f.skrEnv = &envtest.Environment{ // create only once
			ErrorIfCRDPathMissing: true,
		}
		cfg, err := f.skrEnv.Start()
		if err != nil {
			f.baseErr = err
			return
		}
		if cfg == nil {
			f.baseErr = ErrEmptyRestConfig
			return
		}
		var authUser *envtest.AuthenticatedUser
		authUser, err = f.skrEnv.AddUser(envtest.User{
			Name:   "skr-admin-account",
			Groups: []string{"system:masters"},
		}, cfg)
		if err != nil {
			f.baseErr = err
			return
		}
		skrClient, err := client.New(authUser.Config(), client.Options{Scheme: f.scheme})
		if err != nil {
			f.baseErr = err
			return
		}
		f.baseUser = authUser
		f.baseClient = remote.NewClientWithConfig(skrClient, authUser.Config())
	})

	if f.baseErr != nil {
		return f.baseErr
	}
	// Reuse the already created base client for new Kyma names.
	f.clients.Store(kyma.Name, f.baseClient)
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
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.skrEnv == nil { // nothing to stop
		return nil
	}
	err := f.skrEnv.Stop()
	// Reset so a future Init (in another suite) can start a fresh environment.
	f.skrEnv = nil
	f.baseClient = nil
	f.baseUser = nil
	// Allow reuse in another test run within same process by resetting the Once.
	f.baseOnce = sync.Once{}
	return err
}
