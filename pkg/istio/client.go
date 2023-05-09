package istio

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	istioapi "istio.io/api/networking/v1beta1"
	istioclientapi "istio.io/client-go/pkg/apis/networking/v1beta1"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

const (
	firstElementIdx     = 0
	vsDeletionThreshold = 1
	notFoundRouteIndex  = -1
	contractVersion     = "v1"
	prefixFormat        = "/%s/%s/event"
)

var (
	ErrCantFindMatchingGateway    = errors.New("can't find matching Istio Gateway")
	ErrCantFindGatewayServersHost = errors.New("can't find Istio Gateway servers hosts")
)

type Config struct {
	WatcherLocalTesting bool
}

func NewConfig(watcherLocalTesting bool) *Config {
	return &Config{
		WatcherLocalTesting: watcherLocalTesting,
	}
}

type Client struct {
	istioclient.Interface
	cfg           *Config
	eventRecorder record.EventRecorder
	logger        logr.Logger
}

func NewVersionedIstioClient(cfg *rest.Config, config *Config, recorder record.EventRecorder,
	logger logr.Logger,
) (*Client, error) {
	cs, err := istioclient.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{
		Interface:     cs,
		eventRecorder: recorder,
		cfg:           config,
		logger:        logger,
	}, nil
}

func (c *Client) GetVirtualService(ctx context.Context, vsName string) (*istioclientapi.VirtualService, error) {
	virtualService, err := c.NetworkingV1beta1().
		VirtualServices(metav1.NamespaceDefault).
		Get(ctx, vsName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch virtual service %w", err)
	}
	return virtualService, nil
}

func (c *Client) NewVirtualService(ctx context.Context, watcher *v1beta2.Watcher,
) (*istioclientapi.VirtualService, error) {
	if watcher == nil {
		return &istioclientapi.VirtualService{}, nil
	}

	virtualSvc := &istioclientapi.VirtualService{}
	virtualSvc.SetName(watcher.Name)
	virtualSvc.SetNamespace(metav1.NamespaceDefault)

	gateways, err := c.LookupGateways(ctx, watcher)
	if err != nil {
		return nil, err
	}

	addGateways(gateways, virtualSvc)

	if c.cfg.WatcherLocalTesting {
		virtualSvc.Spec.Hosts = []string{"*"}
	} else if err := addHosts(gateways, virtualSvc); err != nil {
		return nil, err
	}

	virtualSvc.Spec.Http = []*istioapi.HTTPRoute{
		PrepareIstioHTTPRouteForCR(watcher),
	}

	return virtualSvc, nil
}

func (c *Client) CreateVirtualService(ctx context.Context, virtualSvc *istioclientapi.VirtualService) error {
	_, err := c.NetworkingV1beta1().
		VirtualServices(metav1.NamespaceDefault).
		Create(ctx, virtualSvc, metav1.CreateOptions{})
	return err
}
func addGateways(gateways []*istioclientapi.Gateway, virtualSvc *istioclientapi.VirtualService) {
	gatewayLists := convertToGatewayList(gateways)
	virtualSvc.Spec.Gateways = gatewayLists
}

func convertToGatewayList(gateways []*istioclientapi.Gateway) []string {
	gatewayLists := make([]string, 0)
	for i := range gateways {
		gatewayLists = append(gatewayLists, client.ObjectKeyFromObject(gateways[i]).String())
	}
	return gatewayLists
}

func addHosts(gateways []*istioclientapi.Gateway, virtualSvc *istioclientapi.VirtualService) error {
	hosts, err := getHosts(gateways)
	if err != nil {
		return err
	}
	virtualSvc.Spec.Hosts = hosts
	return nil
}

// TODO: Create Test to handle hosts
func getHosts(gateways []*istioclientapi.Gateway) ([]string, error) {
	hosts := make([]string, 0)

	for _, g := range gateways {
		servers := g.Spec.Servers
		if len(servers) == 0 || len(servers[0].Hosts) == 0 {
			return nil, fmt.Errorf("for gateway %s: %w",
				client.ObjectKeyFromObject(g).String(),
				ErrCantFindGatewayServersHost)
		}
		for _, s := range servers {
			hosts = append(hosts, s.Hosts...)
		}
	}

	return hosts, nil
}

func (c *Client) LookupGateways(ctx context.Context, watcher *v1beta2.Watcher) ([]*istioclientapi.Gateway, error) {
	selector, err := metav1.LabelSelectorAsSelector(&watcher.Spec.Gateway.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("error converting label selector: %w", err)
	}
	labelSelector := selector.String()
	gateways, err := c.NetworkingV1beta1().
		Gateways(metav1.NamespaceAll).
		List(ctx, metav1.ListOptions{
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

func (c *Client) UpdateVirtualService(ctx context.Context, virtualService *istioclientapi.VirtualService) error {
	_, err := c.NetworkingV1beta1().
		VirtualServices(virtualService.Namespace).
		Update(ctx, virtualService, metav1.UpdateOptions{})
	return err
}

func (c *Client) RemoveVirtualServiceForCR(ctx context.Context, watcherObjKey client.ObjectKey,
) error {
	return c.NetworkingV1beta1().
		VirtualServices(metav1.NamespaceDefault).
		Delete(ctx, watcherObjKey.Name, metav1.DeleteOptions{})
}

func IsRouteConfigEqual(route1 *istioapi.HTTPRoute, route2 *istioapi.HTTPRoute) bool {
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

func PrepareIstioHTTPRouteForCR(obj *v1beta2.Watcher) *istioapi.HTTPRoute {
	return &istioapi.HTTPRoute{
		Name: client.ObjectKeyFromObject(obj).String(),
		Match: []*istioapi.HTTPMatchRequest{
			{
				Uri: &istioapi.StringMatch{
					MatchType: &istioapi.StringMatch_Prefix{ //nolint:nosnakecase
						Prefix: fmt.Sprintf(prefixFormat, contractVersion, obj.GetModuleName()),
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
