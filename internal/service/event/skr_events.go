package event

import (
	"fmt"
	"net/http"
	"strings"

	watcherevent "github.com/kyma-project/runtime-watcher/listener/pkg/v2/event"
	"github.com/kyma-project/runtime-watcher/listener/pkg/v2/types"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/kyma-project/lifecycle-manager/pkg/security"
)

// NewSKREventService creates a new SKR event service with a listener.
func NewSKREventService(mgr manager.Manager, listenerAddr, componentName string, enableDomainNameVerification bool) (*SkrRuntimeEventService, error) {
	// Configure verification function
	var verifyFunc watcherevent.Verify
	if enableDomainNameVerification {
		verifyFunc = security.NewRequestVerifier(mgr.GetClient()).Verify
	} else {
		verifyFunc = func(r *http.Request, watcherEvtObject *types.WatchEvent) error {
			return nil
		}
	}

	// Create a new listener for this address
	runnableListener := watcherevent.NewSKREventListener(
		listenerAddr,
		strings.ToLower(componentName),
		verifyFunc,
	)

	// Add listener to manager as a runnable
	err := mgr.Add(runnableListener)
	if err != nil {
		return nil, fmt.Errorf("failed to add listener to manager: %w", err)
	}

	service := NewSkrRuntimeEventService(runnableListener)

	// Add service to manager for automatic lifecycle management
	err = mgr.Add(service)
	if err != nil {
		return nil, fmt.Errorf("failed to add event service to manager: %w", err)
	}

	return service, nil
}
