package custom

import (
	"context"
	"fmt"
	"strings"

	istioapi "istio.io/api/networking/v1beta1"
	istioclientapi "istio.io/client-go/pkg/apis/networking/v1beta1"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
)

const (
	firstElementIdx    = 0
	contractVersion    = "1"
	virtualServiceName = "kcp-events"
)

type IstioClient struct {
	istioclient.Interface
	// IstioGateway string
	IstioGateway client.ObjectKey
}

func NewVersionedIstioClient(cfg *rest.Config, istioGw string) (*IstioClient, error) {
	gwStrings := strings.Split(istioGw, "/")
	if len(gwStrings) != 2 {
		return nil, fmt.Errorf("error validating gateway config: expected (namespace/name), got: (%s)", istioGw)
	}
	cs, err := istioclient.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &IstioClient{
		Interface: cs,
		IstioGateway: client.ObjectKey{
			Namespace: gwStrings[0],
			Name:      gwStrings[1],
		},
	}, nil
}

func (c *IstioClient) getOrCreateVirtualService(ctx context.Context, obj *v1alpha1.Watcher,
) (*istioclientapi.VirtualService, error) {
	var err error
	var virtualService *istioclientapi.VirtualService
	virtualService, err = c.NetworkingV1beta1().
		VirtualServices(c.IstioGateway.Namespace).
		Get(ctx, virtualServiceName, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		return nil, fmt.Errorf("failed to fetch virtual service %w", err)
	}
	if kerrors.IsNotFound(err) {
		virtualService, err = c.createVirtualService(ctx, obj)
		if err != nil {
			return nil, fmt.Errorf("failed to create virtual service %w", err)
		}
	}
	return virtualService, nil
}

func (c *IstioClient) createVirtualService(ctx context.Context, obj *v1alpha1.Watcher,
) (*istioclientapi.VirtualService, error) {

	_, err := c.NetworkingV1beta1().
		Gateways(c.IstioGateway.Namespace).
		Get(ctx, c.IstioGateway.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting configured istio gateway: %w", err)
	}
	virtualSvc := &istioclientapi.VirtualService{}
	virtualSvc.SetName(virtualServiceName)
	virtualSvc.SetNamespace(c.IstioGateway.Namespace)
	virtualSvc.Spec.Gateways = append(virtualSvc.Spec.Gateways, c.IstioGateway.Name)
	istioHTTPRoute := prepareIstioHTTPRouteForCR(obj)
	virtualSvc.Spec.Http = append(virtualSvc.Spec.Http, istioHTTPRoute)
	return c.NetworkingV1beta1().
		VirtualServices(c.IstioGateway.Namespace).
		Create(ctx, virtualSvc, metav1.CreateOptions{})
}

func (c *IstioClient) updateVirtualService(ctx context.Context, virtualService *istioclientapi.VirtualService) error {
	_, err := c.NetworkingV1beta1().
		VirtualServices(virtualService.Namespace).
		Update(ctx, virtualService, metav1.UpdateOptions{})
	return err
}

func (c *IstioClient) IsListenerHTTPRouteConfigured(ctx context.Context, obj *v1alpha1.Watcher,
) (bool, error) {
	virtualService, err := c.getOrCreateVirtualService(ctx, obj)
	if err != nil {
		return false, err
	}
	if len(virtualService.Spec.Http) == 0 {
		return false, nil
	}

	for idx, route := range virtualService.Spec.Http {
		if route.Name == obj.GetModuleName() {
			istioHTTPRoute := prepareIstioHTTPRouteForCR(obj)
			return isRouteConfigEqual(virtualService.Spec.Http[idx], istioHTTPRoute), nil
		}
	}

	return false, nil
}

func (c *IstioClient) IsListenerHTTPRoutesEmpty(ctx context.Context) (bool, error) {
	virtualService, err := c.NetworkingV1beta1().
		VirtualServices(metav1.NamespaceDefault).
		Get(ctx, virtualServiceName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	if len(virtualService.Spec.Http) == 0 {
		return true, nil
	}
	return false, nil
}

func (c *IstioClient) UpdateVirtualServiceConfig(ctx context.Context, vsObjectKey client.ObjectKey,
	obj *v1alpha1.Watcher,
) error {
	virtualService, err := c.getOrCreateVirtualService(ctx, obj)
	if err != nil {
		return err
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
	virtualService, err := c.getOrCreateVirtualService(ctx, obj)
	if err != nil {
		return err
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
