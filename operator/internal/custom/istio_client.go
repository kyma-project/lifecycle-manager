package custom

import (
	"context"
	"fmt"

	istioapi "istio.io/api/networking/v1beta1"
	istioclientapi "istio.io/client-go/pkg/apis/networking/v1beta1"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
)

const (
	firstElementIdx     = 0
	vsDeletionThreshold = 1
	contractVersion     = "v1"
	virtualServiceName  = "kcp-events"
	gatewayName         = "lifecycle-manager-kyma-gateway"
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

type customClientErr struct {
	Err        error
	IsNotFound bool
}

func (c *IstioClient) getVirtualService(ctx context.Context) (*istioclientapi.VirtualService, *customClientErr) {
	virtualService, err := c.NetworkingV1beta1().
		VirtualServices(metav1.NamespaceDefault).
		Get(ctx, virtualServiceName, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		return nil, &customClientErr{
			Err:        fmt.Errorf("failed to fetch virtual service %w", err),
			IsNotFound: false,
		}
	}
	if apierrors.IsNotFound(err) {
		return nil, &customClientErr{
			Err:        err,
			IsNotFound: true,
		}
	}
	return virtualService, nil
}

func (c *IstioClient) createVirtualService(ctx context.Context, watcher *v1alpha1.Watcher,
) (*istioclientapi.VirtualService, error) {
	if watcher == nil {
		return &istioclientapi.VirtualService{}, nil
	}
	_, err := c.NetworkingV1beta1().
		Gateways(metav1.NamespaceDefault).
		Get(ctx, gatewayName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting configured istio gateway: %w", err)
	}
	virtualSvc := &istioclientapi.VirtualService{}
	virtualSvc.SetName(virtualServiceName)
	virtualSvc.SetNamespace(metav1.NamespaceDefault)
	virtualSvc.Spec.Gateways = append(virtualSvc.Spec.Gateways, gatewayName)
	virtualSvc.Spec.Hosts = append(virtualSvc.Spec.Hosts, "*")
	virtualSvc.Spec.Http = []*istioapi.HTTPRoute{
		prepareIstioHTTPRouteForCR(watcher),
	}
	return c.NetworkingV1beta1().
		VirtualServices(metav1.NamespaceDefault).
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
	virtualService, customErr := c.getVirtualService(ctx)
	if customErr != nil {
		return false, customErr.Err
	}
	if len(virtualService.Spec.Http) == 0 {
		return false, nil
	}

	for idx, route := range virtualService.Spec.Http {
		if route.Name == client.ObjectKeyFromObject(obj).String() {
			istioHTTPRoute := prepareIstioHTTPRouteForCR(obj)
			return isRouteConfigEqual(virtualService.Spec.Http[idx], istioHTTPRoute), nil
		}
	}

	return false, nil
}

func (c *IstioClient) IsVsDeleted(ctx context.Context) (bool, error) {
	_, err := c.NetworkingV1beta1().
		VirtualServices(metav1.NamespaceDefault).
		Get(ctx, virtualServiceName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return true, nil
	}
	return false, err
}

func (c *IstioClient) UpdateVirtualServiceConfig(ctx context.Context, watcher *v1alpha1.Watcher,
) error {
	var err error
	var customErr *customClientErr
	var virtualService *istioclientapi.VirtualService
	virtualService, customErr = c.getVirtualService(ctx)
	if customErr != nil && customErr.IsNotFound {
		_, err = c.createVirtualService(ctx, watcher)
		if err != nil {
			return fmt.Errorf("failed to create virtual service %w", err)
		}
		return nil
	}
	// lookup cr config
	routeIdx := lookupHTTPRouteByObjectKey(virtualService.Spec.Http, client.ObjectKeyFromObject(watcher))
	if routeIdx != -1 {
		istioHTTPRoute := prepareIstioHTTPRouteForCR(watcher)
		if isRouteConfigEqual(virtualService.Spec.Http[routeIdx], istioHTTPRoute) {
			return nil
		}
		virtualService.Spec.Http[routeIdx] = prepareIstioHTTPRouteForCR(watcher)
		return c.updateVirtualService(ctx, virtualService)
	}
	// if route doesn't exist already append it to the route list
	istioHTTPRoute := prepareIstioHTTPRouteForCR(watcher)
	virtualService.Spec.Http = append(virtualService.Spec.Http, istioHTTPRoute)
	return c.updateVirtualService(ctx, virtualService)
}

func (c *IstioClient) RemoveVirtualServiceConfigForCR(ctx context.Context, watcherObjKey client.ObjectKey,
) error {
	virtualService, customErr := c.getVirtualService(ctx)
	if customErr != nil {
		return nil
	}
	if len(virtualService.Spec.Http) <= vsDeletionThreshold {
		// last http route is being deleted: remove the virtual service resource
		return c.NetworkingV1beta1().
			VirtualServices(metav1.NamespaceDefault).
			Delete(ctx, virtualServiceName, metav1.DeleteOptions{})
	}

	routeIdx := lookupHTTPRouteByObjectKey(virtualService.Spec.Http, watcherObjKey)
	if routeIdx == -1 {
		return nil
	}
	l := len(virtualService.Spec.Http)
	copy(virtualService.Spec.Http[routeIdx:], virtualService.Spec.Http[routeIdx+1:])
	virtualService.Spec.Http[l-1] = nil
	virtualService.Spec.Http = virtualService.Spec.Http[:l-1]
	return c.updateVirtualService(ctx, virtualService)
}

func lookupHTTPRouteByObjectKey(routes []*istioapi.HTTPRoute, watcherObjKey client.ObjectKey) int {
	if len(routes) == 0 {
		return -1
	}
	for idx, route := range routes {
		if route.Name == watcherObjKey.String() {
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
		Name: client.ObjectKeyFromObject(obj).String(),
		Match: []*istioapi.HTTPMatchRequest{
			{
				Uri: &istioapi.StringMatch{
					MatchType: &istioapi.StringMatch_Prefix{ //nolint:nosnakecase
						Prefix: fmt.Sprintf("/%s/%s/event", contractVersion, obj.GetModuleName()),
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
