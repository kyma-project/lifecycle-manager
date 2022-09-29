package custom

import (
	"context"
	"fmt"

	istioapi "istio.io/api/networking/v1beta1"
	istioclientapi "istio.io/client-go/pkg/apis/networking/v1beta1"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
)

const (
	firstElementIdx = 0
	contractVersion = "1"
)

type IstioClient struct {
	istioclient.Interface
}

func NewVersionedIstioClient(cfg *rest.Config) (*IstioClient, error) {
	cs, err := istioclient.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &IstioClient{
		Interface: cs,
	}, nil
}

func (c *IstioClient) getVirtualService(ctx context.Context, vsObjectKey client.ObjectKey,
) *istioclientapi.VirtualService {
	virtualService, err := c.NetworkingV1beta1().
		VirtualServices(vsObjectKey.Namespace).
		Get(ctx, vsObjectKey.Name, metav1.GetOptions{})
	if err != nil {
		return nil
	}
	return virtualService
}

func (c *IstioClient) updateVirtualService(ctx context.Context, virtualService *istioclientapi.VirtualService) error {
	_, err := c.NetworkingV1beta1().
		VirtualServices(virtualService.Namespace).
		Update(ctx, virtualService, metav1.UpdateOptions{})
	return err
}

func (c *IstioClient) IsListenerHTTPRouteConfigured(ctx context.Context, vsObjectKey client.ObjectKey,
	obj *v1alpha1.Watcher,
) bool {
	virtualService := c.getVirtualService(ctx, vsObjectKey)
	if virtualService == nil {
		return false
	}
	if len(virtualService.Spec.Http) == 0 {
		return false
	}

	for idx, route := range virtualService.Spec.Http {
		if route.Name == obj.GetModuleName() {
			istioHTTPRoute := prepareIstioHTTPRouteForCR(obj)
			return isRouteConfigEqual(virtualService.Spec.Http[idx], istioHTTPRoute)
		}
	}

	return false
}

func (c *IstioClient) IsListenerHTTPRoutesEmpty(ctx context.Context, vsObjectKey client.ObjectKey) bool {
	virtualService := c.getVirtualService(ctx, vsObjectKey)
	if virtualService == nil {
		return false
	}
	if len(virtualService.Spec.Http) == 0 {
		return true
	}
	return false
}

func (c *IstioClient) UpdateVirtualServiceConfig(ctx context.Context, vsObjectKey client.ObjectKey,
	obj *v1alpha1.Watcher,
) error {
	virtualService := c.getVirtualService(ctx, vsObjectKey)
	if virtualService == nil {
		return fmt.Errorf("failed to get virtual service")
	}
	// lookup cr config
	routeIdx := lookupHTTPRouteByName(virtualService.Spec.Http, obj.GetModuleName())
	if routeIdx != -1 {
		istioHTTPRoute := prepareIstioHTTPRouteForCR(obj)
		if isRouteConfigEqual(virtualService.Spec.Http[routeIdx], istioHTTPRoute) {
			return nil
		}
		virtualService.Spec.Http[routeIdx] = prepareIstioHTTPRouteForCR(obj)
		return c.updateVirtualService(ctx, virtualService)
	}
	// if route doesn't exist already append it to the route list
	istioHTTPRoute := prepareIstioHTTPRouteForCR(obj)
	virtualService.Spec.Http = append(virtualService.Spec.Http, istioHTTPRoute)
	return c.updateVirtualService(ctx, virtualService)
}

func (c *IstioClient) RemoveVirtualServiceConfigForCR(ctx context.Context, vsObjectKey client.ObjectKey,
	obj *v1alpha1.Watcher,
) error {
	virtualService := c.getVirtualService(ctx, vsObjectKey)
	if virtualService == nil {
		return fmt.Errorf("failed to get virtual service")
	}
	if len(virtualService.Spec.Http) == 0 {
		// nothing to remove
		return nil
	}

	routeIdx := lookupHTTPRouteByName(virtualService.Spec.Http, obj.GetModuleName())
	if routeIdx != -1 {
		l := len(virtualService.Spec.Http)
		copy(virtualService.Spec.Http[routeIdx:], virtualService.Spec.Http[routeIdx+1:])
		virtualService.Spec.Http[l-1] = nil
		virtualService.Spec.Http = virtualService.Spec.Http[:l-1]
	}
	return c.updateVirtualService(ctx, virtualService)
}

func lookupHTTPRouteByName(routes []*istioapi.HTTPRoute, name string) int {
	if len(routes) == 0 {
		return -1
	}
	for idx, route := range routes {
		if route.Name == name {
			return idx
		}
	}
	return -1
}

func isRouteConfigEqual(route1 *istioapi.HTTPRoute, route2 *istioapi.HTTPRoute) bool {
	if route1.Match[firstElementIdx].Uri.MatchType.(*istioapi.StringMatch_Prefix).Prefix != //nolint:nosnakecase
		route2.Match[firstElementIdx].Uri.MatchType.(*istioapi.StringMatch_Prefix).Prefix { //nolint:nosnakecase
		return false
	}

	if route1.Route[firstElementIdx].Destination.Host !=
		route2.Route[firstElementIdx].Destination.Host {
		return false
	}

	if route1.Route[firstElementIdx].Destination.Port.Number !=
		route2.Route[firstElementIdx].Destination.Port.Number {
		return false
	}

	return true
}

func prepareIstioHTTPRouteForCR(obj *v1alpha1.Watcher) *istioapi.HTTPRoute {
	return &istioapi.HTTPRoute{
		Name: obj.GetModuleName(),
		Match: []*istioapi.HTTPMatchRequest{
			{
				Uri: &istioapi.StringMatch{
					MatchType: &istioapi.StringMatch_Prefix{ //nolint:nosnakecase
						Prefix: fmt.Sprintf("/v%s/%s/event", contractVersion, obj.GetModuleName()),
					},
				},
			},
		},
		Route: []*istioapi.HTTPRouteDestination{
			{
				Destination: &istioapi.Destination{
					Host: destinationHost(obj.Spec.ServiceInfo.Name, obj.Spec.ServiceInfo.Namespace),
					Port: &istioapi.PortSelector{
						Number: uint32(obj.Spec.ServiceInfo.Port),
					},
				},
			},
		},
	}
}

func destinationHost(serviceName, serviceNamespace string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, serviceNamespace)
}
