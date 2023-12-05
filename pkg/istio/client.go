package istio

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	istioapiv1beta1 "istio.io/api/networking/v1beta1"
	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

const (
	firstElementIdx = 0
	contractVersion = "v1"
	prefixFormat    = "/%s/%s/event"
)

var (
	ErrCantFindMatchingGateway    = errors.New("can't find matching Istio Gateway")
	ErrCantFindGatewayServersHost = errors.New("can't find Istio Gateway servers hosts")
)

type Client struct {
	istioclient.Interface
	eventRecorder record.EventRecorder
	logger        logr.Logger
}

func NewVersionedIstioClient(cfg *rest.Config, recorder record.EventRecorder,
	logger logr.Logger,
) (*Client, error) {
	cs, err := istioclient.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create istio client from config: %w", err)
	}
	return &Client{
		Interface:     cs,
		eventRecorder: recorder,
		logger:        logger,
	}, nil
}

func (c *Client) GetVirtualService(ctx context.Context, vsName string) (*istioclientapiv1beta1.VirtualService, error) {
	virtualService, err := c.NetworkingV1beta1().
		VirtualServices(apimetav1.NamespaceDefault).
		Get(ctx, vsName, apimetav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch virtual service %w", err)
	}
	return virtualService, nil
}

func (c *Client) ListVirtualServices(ctx context.Context) (*istioclientapiv1beta1.VirtualServiceList, error) {
	virtualServiceList, err := c.NetworkingV1beta1().
		VirtualServices(apimetav1.NamespaceDefault).
		List(ctx, apimetav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list virtual services %w", err)
	}
	return virtualServiceList, nil
}

func (c *Client) NewVirtualService(ctx context.Context, watcher *v1beta2.Watcher,
) (*istioclientapiv1beta1.VirtualService, error) {
	if watcher == nil {
		return &istioclientapiv1beta1.VirtualService{}, nil
	}

	virtualSvc := &istioclientapiv1beta1.VirtualService{}
	virtualSvc.SetName(watcher.Name)
	virtualSvc.SetNamespace(apimetav1.NamespaceDefault)

	gateways, err := c.LookupGateways(ctx, watcher)
	if err != nil {
		return nil, err
	}

	addGateways(gateways, virtualSvc)

	if err := addHosts(gateways, virtualSvc); err != nil {
		return nil, err
	}

	virtualSvc.Spec.Http = []*istioapiv1beta1.HTTPRoute{
		PrepareIstioHTTPRouteForCR(watcher),
	}

	return virtualSvc, nil
}

func (c *Client) CreateVirtualService(ctx context.Context, virtualSvc *istioclientapiv1beta1.VirtualService) error {
	_, err := c.NetworkingV1beta1().
		VirtualServices(apimetav1.NamespaceDefault).
		Create(ctx, virtualSvc, apimetav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create istio virtual service: %w", err)
	}
	return nil
}

func addGateways(gateways []*istioclientapiv1beta1.Gateway, virtualSvc *istioclientapiv1beta1.VirtualService) {
	gatewayLists := convertToGatewayList(gateways)
	virtualSvc.Spec.Gateways = gatewayLists
}

func convertToGatewayList(gateways []*istioclientapiv1beta1.Gateway) []string {
	gatewayLists := make([]string, 0)
	for i := range gateways {
		gatewayLists = append(gatewayLists, client.ObjectKeyFromObject(gateways[i]).String())
	}
	return gatewayLists
}

func addHosts(gateways []*istioclientapiv1beta1.Gateway, virtualSvc *istioclientapiv1beta1.VirtualService) error {
	hosts, err := getHosts(gateways)
	if err != nil {
		return err
	}
	virtualSvc.Spec.Hosts = hosts
	return nil
}

func getHosts(gateways []*istioclientapiv1beta1.Gateway) ([]string, error) {
	hosts := make([]string, 0)

	for _, g := range gateways {
		servers := g.Spec.GetServers()
		if len(servers) == 0 || len(servers[0].GetHosts()) == 0 {
			return nil, fmt.Errorf("for gateway %s: %w",
				client.ObjectKeyFromObject(g).String(),
				ErrCantFindGatewayServersHost)
		}
		for _, s := range servers {
			hosts = append(hosts, s.GetHosts()...)
		}
	}

	return hosts, nil
}

func (c *Client) LookupGateways(ctx context.Context, watcher *v1beta2.Watcher) ([]*istioclientapiv1beta1.Gateway,
	error,
) {
	selector, err := apimetav1.LabelSelectorAsSelector(&watcher.Spec.Gateway.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("error converting label selector: %w", err)
	}
	labelSelector := selector.String()
	gateways, err := c.NetworkingV1beta1().
		Gateways(apimetav1.NamespaceAll).
		List(ctx, apimetav1.ListOptions{
			LabelSelector: labelSelector,
		})
	if err != nil {
		return nil, fmt.Errorf("error looking up Istio gateway with the label selector %q: %w",
			labelSelector, err)
	}

	if len(gateways.Items) == 0 {
		c.eventRecorder.Event(watcher, "Warning", "WatcherGatewayNotFound",
			"Watcher: Gateway for the VirtualService not found")
		return nil, fmt.Errorf("%w. Label selector: %q", ErrCantFindMatchingGateway, labelSelector)
	}

	return gateways.Items, nil
}

func (c *Client) UpdateVirtualService(ctx context.Context, virtualService,
	virtualServiceRemote *istioclientapiv1beta1.VirtualService,
) error {
	virtualServiceRemote.Name = virtualService.Name
	virtualServiceRemote.Namespace = virtualService.Namespace
	virtualService.Spec.DeepCopyInto(&virtualServiceRemote.Spec)

	_, err := c.NetworkingV1beta1().
		VirtualServices(virtualServiceRemote.Namespace).
		Update(ctx, virtualServiceRemote, apimetav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update istio virtual service: %w", err)
	}
	return nil
}

func (c *Client) RemoveVirtualServiceForCR(ctx context.Context, watcherObjKey client.ObjectKey,
) error {
	err := c.NetworkingV1beta1().
		VirtualServices(apimetav1.NamespaceDefault).
		Delete(ctx, watcherObjKey.Name, apimetav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete virtual service for cr: %w", err)
	}
	return nil
}

func IsRouteConfigEqual(route1 *istioapiv1beta1.HTTPRoute, route2 *istioapiv1beta1.HTTPRoute) bool {
	stringMatch1, ok := route1.GetMatch()[firstElementIdx].GetUri().GetMatchType().(*istioapiv1beta1.StringMatch_Prefix)
	if !ok {
		return false
	}
	stringMatch2, ok := route2.GetMatch()[firstElementIdx].GetUri().GetMatchType().(*istioapiv1beta1.StringMatch_Prefix)
	if !ok {
		return false
	}

	if stringMatch1.Prefix != stringMatch2.Prefix {
		return false
	}

	if route1.GetRoute()[firstElementIdx].GetDestination().GetHost() !=
		route2.GetRoute()[firstElementIdx].GetDestination().GetHost() {
		return false
	}

	if route1.GetRoute()[firstElementIdx].GetDestination().GetPort().GetNumber() !=
		route2.GetRoute()[firstElementIdx].GetDestination().GetPort().GetNumber() {
		return false
	}

	return true
}

func PrepareIstioHTTPRouteForCR(obj *v1beta2.Watcher) *istioapiv1beta1.HTTPRoute {
	return &istioapiv1beta1.HTTPRoute{
		Name: client.ObjectKeyFromObject(obj).String(),
		Match: []*istioapiv1beta1.HTTPMatchRequest{
			{
				Uri: &istioapiv1beta1.StringMatch{
					MatchType: &istioapiv1beta1.StringMatch_Prefix{
						//nolint:nosnakecase // external type
						Prefix: fmt.Sprintf(prefixFormat, contractVersion, obj.GetModuleName()),
					},
				},
			},
		},
		Route: []*istioapiv1beta1.HTTPRouteDestination{
			{
				Destination: &istioapiv1beta1.Destination{
					Host: destinationHost(obj.Spec.ServiceInfo.Name, obj.Spec.ServiceInfo.Namespace),
					Port: &istioapiv1beta1.PortSelector{
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
