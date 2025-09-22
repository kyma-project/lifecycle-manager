package istio

import (
	"fmt"

	istioapiv1beta1 "istio.io/api/networking/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func NewHTTPRoute(watcher *v1beta2.Watcher) (*istioapiv1beta1.HTTPRoute, error) {
	if err := validateArgumentsForNewHTTPRoute(watcher); err != nil {
		return nil, err
	}

	return &istioapiv1beta1.HTTPRoute{
		Name: client.ObjectKeyFromObject(watcher).String(),
		Match: []*istioapiv1beta1.HTTPMatchRequest{
			{
				Uri: &istioapiv1beta1.StringMatch{
					MatchType: &istioapiv1beta1.StringMatch_Prefix{
						Prefix: fmt.Sprintf(prefixFormat, contractVersion, watcher.GetManagerName()),
					},
				},
			},
		},
		Route: []*istioapiv1beta1.HTTPRouteDestination{
			{
				Destination: &istioapiv1beta1.Destination{
					Host: destinationHost(watcher.Spec.ServiceInfo.Name, watcher.Spec.ServiceInfo.Namespace),
					Port: &istioapiv1beta1.PortSelector{Number: uint32(watcher.Spec.ServiceInfo.Port)}, //nolint:gosec,revive // see validation of port range below
				},
			},
		},
	}, nil
}

const (
	minPort = 1
	maxPort = 65535
)

func validateArgumentsForNewHTTPRoute(watcher *v1beta2.Watcher) error {
	if watcher == nil {
		return fmt.Errorf("watcher must not be nil: %w", ErrInvalidArgument)
	}

	if watcher.GetName() == "" {
		return fmt.Errorf("watcher.Name must not be empty: %w", ErrInvalidArgument)
	}

	if watcher.GetNamespace() == "" {
		return fmt.Errorf("watcher.Namespace must not be empty: %w", ErrInvalidArgument)
	}

	if watcher.GetManagerName() == "" {
		return fmt.Errorf("unable to GetManagerName(): %w", ErrInvalidArgument)
	}

	if watcher.Spec.ServiceInfo.Name == "" {
		return fmt.Errorf("watcher.Spec.ServiceInfo.Name must not be empty: %w", ErrInvalidArgument)
	}

	if watcher.Spec.ServiceInfo.Namespace == "" {
		return fmt.Errorf("watcher.Spec.ServiceInfo.Namespace must not be empty: %w", ErrInvalidArgument)
	}

	if watcher.Spec.ServiceInfo.Port == 0 {
		return fmt.Errorf("watcher.Spec.ServiceInfo.Port must not be 0 as it is reserved: %w", ErrInvalidArgument)
	}

	if watcher.Spec.ServiceInfo.Port < minPort || watcher.Spec.ServiceInfo.Port > maxPort {
		return fmt.Errorf("watcher.Spec.ServiceInfo.Port must be between %d and %d: %w", minPort, maxPort,
			ErrInvalidArgument)
	}

	return nil
}
