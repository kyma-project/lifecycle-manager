package istio

import (
	"fmt"

	istioapiv1beta1 "istio.io/api/networking/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func NewHTTPRoute(watcher *v1beta2.Watcher) (*istioapiv1beta1.HTTPRoute, error) {
	if watcher == nil {
		return nil, fmt.Errorf("%w watcher", ErrInvalidArgument)
	}

	if watcher.GetName() == "" {
		return nil, fmt.Errorf("%w watcher.Name", ErrInvalidArgument)
	}

	if watcher.GetNamespace() == "" {
		return nil, fmt.Errorf("%w watcher.Namespace", ErrInvalidArgument)
	}

	if watcher.GetModuleName() == "" {
		return nil, fmt.Errorf("%w watcher.GetModuleName()", ErrInvalidArgument)
	}

	if watcher.Spec.ServiceInfo.Name == "" {
		return nil, fmt.Errorf("%w watcher.Spec.ServiceInfo.Name", ErrInvalidArgument)
	}

	if watcher.Spec.ServiceInfo.Namespace == "" {
		return nil, fmt.Errorf("%w watcher.Spec.ServiceInfo.Namespace", ErrInvalidArgument)
	}

	// 0 is the zero value of int64 and further a reserved port => consider it invalid
	if watcher.Spec.ServiceInfo.Port == 0 {
		return nil, fmt.Errorf("%w watcher.Spec.ServiceInfo.Port", ErrInvalidArgument)
	}

	return &istioapiv1beta1.HTTPRoute{
		Name: client.ObjectKeyFromObject(watcher).String(),
		Match: []*istioapiv1beta1.HTTPMatchRequest{
			{
				Uri: &istioapiv1beta1.StringMatch{
					MatchType: &istioapiv1beta1.StringMatch_Prefix{
						//nolint:nosnakecase // external type
						Prefix: fmt.Sprintf(prefixFormat, contractVersion, watcher.GetModuleName()),
					},
				},
			},
		},
		Route: []*istioapiv1beta1.HTTPRouteDestination{
			{
				Destination: &istioapiv1beta1.Destination{
					Host: destinationHost(watcher.Spec.ServiceInfo.Name, watcher.Spec.ServiceInfo.Namespace),
					Port: &istioapiv1beta1.PortSelector{
						Number: uint32(watcher.Spec.ServiceInfo.Port),
					},
				},
			},
		},
	}, nil
}
