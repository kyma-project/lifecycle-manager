package istio

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	istioapi "istio.io/api/networking/v1beta1"
	istioclientapi "istio.io/client-go/pkg/apis/networking/v1beta1"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
)

const (
	firstElementIdx     = 0
	vsDeletionThreshold = 1
	contractVersion     = "v1"
)

var (
	errNoGatewayConfigured     = errors.New("error processing Watcher: No istio gateway configured")
	errCantFindMatchingGateway = errors.New("can't find matching Istio Gateway")
)

type Config struct {
	VirtualServiceName string
	Gateway            v1alpha1.GatewayConfig
}

func NewConfig(vsn, gnsn string, gsel *metav1.LabelSelector) Config {
	return Config{
		VirtualServiceName: vsn,
		Gateway: v1alpha1.GatewayConfig{
			NamespacedName: gnsn,
			LabelSelector:  gsel,
		},
	}
}

type Client struct {
	istioclient.Interface
	config        Config
	eventRecorder record.EventRecorder
	logger        logr.Logger
}

func NewVersionedIstioClient(cfg *rest.Config, config Config, recorder record.EventRecorder,
	logger logr.Logger,
) (*Client, error) {
	cs, err := istioclient.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{
		Interface:     cs,
		eventRecorder: recorder,
		config:        config,
		logger:        logger,
	}, nil
}

type customClientErr struct {
	Err        error
	IsNotFound bool
}

func (c *Client) getVirtualService(ctx context.Context) (*istioclientapi.VirtualService, *customClientErr) {
	virtualService, err := c.NetworkingV1beta1().
		VirtualServices(metav1.NamespaceDefault).
		Get(ctx, c.config.VirtualServiceName, metav1.GetOptions{})
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

func (c *Client) createVirtualService(ctx context.Context, watcher *v1alpha1.Watcher,
) (*istioclientapi.VirtualService, error) {
	if watcher == nil {
		return &istioclientapi.VirtualService{}, nil
	}

	gateway, err := c.lookupGateway(ctx, watcher)
	if err != nil {
		return nil, err
	}

	virtualSvc := &istioclientapi.VirtualService{}
	virtualSvc.SetName(c.config.VirtualServiceName)
	virtualSvc.SetNamespace(metav1.NamespaceDefault)
	virtualSvc.Spec.Gateways = append(virtualSvc.Spec.Gateways, client.ObjectKeyFromObject(gateway).String())
	virtualSvc.Spec.Hosts = append(virtualSvc.Spec.Hosts, "*")
	virtualSvc.Spec.Http = []*istioapi.HTTPRoute{
		prepareIstioHTTPRouteForCR(watcher),
	}

	return c.NetworkingV1beta1().
		VirtualServices(metav1.NamespaceDefault).
		Create(ctx, virtualSvc, metav1.CreateOptions{})
}

func (c *Client) lookupGateway(ctx context.Context, watcher *v1alpha1.Watcher) (*istioclientapi.Gateway, error) {
	gName, gSel := mergeSelectorCfg(c.config.Gateway, watcher.Spec.Gateway)

	if gName == "" && gSel == nil {
		c.eventRecorder.Event(watcher, "Warning", "WatcherGatewayNotConfigured",
			"Watcher: Gateway for the VirtualService not configured")
		return nil, errNoGatewayConfigured
	}

	ls, err := metav1.LabelSelectorAsSelector(gSel)
	if err != nil {
		return nil, fmt.Errorf("error converting label selector: %w", err)
	}
	lss := ls.String()

	// Gateway namespacedName takes precedence as it is more specific than label selector lookup
	if gName != "" {
		namespace, name := splitOnSlash(gName)
		gateway, err := c.NetworkingV1beta1().
			Gateways(namespace).
			Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error getting configured istio gateway: %w", err)
		}

		return gateway, nil
	}

	// fallback to label selector
	lo := metav1.ListOptions{
		LabelSelector: lss,
	}
	gateways, err := c.NetworkingV1beta1().
		Gateways(metav1.NamespaceAll).
		List(ctx, lo)
	if err != nil {
		return nil, fmt.Errorf("error looking up Istio gateway with the label selector %q: %w", lss, err)
	}

	if len(gateways.Items) == 0 {
		c.eventRecorder.Event(watcher, "Warning", "WatcherGatewayNotFound",
			"Watcher: Gateway for the VirtualService not found")
		return nil, fmt.Errorf("%w. Label selector: %q", errCantFindMatchingGateway, lss)
	}

	if len(gateways.Items) > 1 {
		gwKey := client.ObjectKeyFromObject(gateways.Items[0])
		c.eventRecorder.Event(watcher, "Warning", "WatcherMultipleGatewaysFound",
			fmt.Sprintf("Watcher: Found multiple matching Istio Gateways for the VirtualService. Selecting %s", gwKey.String()))
		c.logger.Info("Warning: Found multiple matching Istio gateways. Selecting the first one",
			"labelSelector", lss, "match count", len(gateways.Items), "selected", gwKey.String())
	}

	return gateways.Items[0], nil
}

func (c *Client) updateVirtualService(ctx context.Context, virtualService *istioclientapi.VirtualService) error {
	_, err := c.NetworkingV1beta1().
		VirtualServices(virtualService.Namespace).
		Update(ctx, virtualService, metav1.UpdateOptions{})
	return err
}

func (c *Client) IsListenerHTTPRouteConfigured(ctx context.Context, obj *v1alpha1.Watcher,
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

func (c *Client) IsVsDeleted(ctx context.Context) (bool, error) {
	_, err := c.NetworkingV1beta1().
		VirtualServices(metav1.NamespaceDefault).
		Get(ctx, c.config.VirtualServiceName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return true, nil
	}
	return false, err
}

func (c *Client) UpdateVirtualServiceConfig(ctx context.Context, watcher *v1alpha1.Watcher,
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

func (c *Client) RemoveVirtualServiceConfigForCR(ctx context.Context, watcherObjKey client.ObjectKey,
) error {
	virtualService, customErr := c.getVirtualService(ctx)
	if customErr != nil {
		return nil
	}
	if len(virtualService.Spec.Http) <= vsDeletionThreshold {
		// last http route is being deleted: remove the virtual service resource
		return c.NetworkingV1beta1().
			VirtualServices(metav1.NamespaceDefault).
			Delete(ctx, c.config.VirtualServiceName, metav1.DeleteOptions{})
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

func splitOnSlash(s string) (string, string) {
	res := strings.Split(s, "/")
	return res[0], res[1]
}

func mergeSelectorCfg(fromConfig v1alpha1.GatewayConfig, fromWatcher *v1alpha1.GatewayConfig) (
	string, *metav1.LabelSelector,
) {
	gName := fromConfig.NamespacedName
	gSel := fromConfig.LabelSelector

	if fromWatcher != nil {
		if fromWatcher.NamespacedName != "" {
			gName = fromWatcher.NamespacedName
		}

		if fromWatcher.LabelSelector != nil {
			gSel = fromWatcher.LabelSelector
		}
	}

	return gName, gSel
}
