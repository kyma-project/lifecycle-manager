package istio

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/pkg/log"

	corev1 "k8s.io/api/core/v1"

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
	prefixFormat        = "/%s/%s/event"
)

var (
	ErrCantFindMatchingGateway    = errors.New("can't find matching Istio Gateway")
	ErrCantFindGatewayServersHost = errors.New("can't find Istio Gateway servers hosts")
)

type Config struct {
	VirtualServiceName  string
	WatcherLocalTesting bool
	*CACertOptions
}

type CACertOptions struct {
	WatcherRootCertificateName      string
	WatcherRootCertificateNamespace string
	IstioCertificateNamespace       string
}

func NewConfig(vsn string, options *CACertOptions, watcherLocalTesting bool) Config {
	return Config{
		VirtualServiceName:  vsn,
		WatcherLocalTesting: watcherLocalTesting,
		CACertOptions:       options,
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

func (c *Client) GetVirtualService(ctx context.Context) (*istioclientapi.VirtualService, error) {
	virtualService, err := c.NetworkingV1beta1().
		VirtualServices(metav1.NamespaceDefault).
		Get(ctx, c.config.VirtualServiceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch virtual service %w", err)
	}
	return virtualService, nil
}

func (c *Client) CreateVirtualService(ctx context.Context, watcher *v1alpha1.Watcher,
) (*istioclientapi.VirtualService, error) {
	if watcher == nil {
		return &istioclientapi.VirtualService{}, nil
	}

	virtualSvc := &istioclientapi.VirtualService{}
	virtualSvc.SetName(c.config.VirtualServiceName)
	virtualSvc.SetNamespace(metav1.NamespaceDefault)

	gateways, err := c.LookupGateways(ctx, watcher)
	if err != nil {
		return nil, err
	}

	addGateways(gateways, virtualSvc)

	if c.config.WatcherLocalTesting {
		virtualSvc.Spec.Hosts = []string{"*"}
	} else if err := addHosts(gateways, virtualSvc); err != nil {
		return nil, err
	}

	virtualSvc.Spec.Http = []*istioapi.HTTPRoute{
		prepareIstioHTTPRouteForCR(watcher),
	}

	return c.NetworkingV1beta1().
		VirtualServices(metav1.NamespaceDefault).
		Create(ctx, virtualSvc, metav1.CreateOptions{})
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

func getHosts(gateways []*istioclientapi.Gateway) ([]string, error) {
	uniqueHostsMap := make(map[string]bool)
	for i := range gateways {
		gateway := gateways[i]
		servers := gateway.Spec.Servers
		if len(servers) == 0 || len(servers[0].Hosts) == 0 {
			return nil, fmt.Errorf("for gateway %s: %w",
				client.ObjectKeyFromObject(gateway).String(),
				ErrCantFindGatewayServersHost)
		}
		// The first Hosts entry must be the listener host, use map to filter out duplicate hosts
		uniqueHostsMap[servers[0].Hosts[0]] = true
	}
	hosts := make([]string, 0)
	for host := range uniqueHostsMap {
		hosts = append(hosts, host)
	}
	return hosts, nil
}

func (c *Client) LookupGateways(ctx context.Context, watcher *v1alpha1.Watcher) ([]*istioclientapi.Gateway, error) {
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

func (c *Client) updateVirtualService(ctx context.Context, virtualService *istioclientapi.VirtualService) error {
	_, err := c.NetworkingV1beta1().
		VirtualServices(virtualService.Namespace).
		Update(ctx, virtualService, metav1.UpdateOptions{})
	return err
}

func (c *Client) IsListenerHTTPRouteConfigured(ctx context.Context, watcher *v1alpha1.Watcher,
) (bool, error) {
	virtualService, err := c.GetVirtualService(ctx)
	if err != nil {
		return false, err
	}
	if len(virtualService.Spec.Http) == 0 {
		return false, nil
	}

	for idx, route := range virtualService.Spec.Http {
		if route.Name == client.ObjectKeyFromObject(watcher).String() {
			istioHTTPRoute := prepareIstioHTTPRouteForCR(watcher)
			return isRouteConfigEqual(virtualService.Spec.Http[idx], istioHTTPRoute), nil
		}
	}

	return false, nil
}

func (c *Client) IsVirtualServiceDeleted(ctx context.Context) (bool, error) {
	_, err := c.NetworkingV1beta1().
		VirtualServices(metav1.NamespaceDefault).
		Get(ctx, c.config.VirtualServiceName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return true, nil
	}
	return false, err
}

func (c *Client) UpdateVirtualServiceConfig(ctx context.Context, watcher *v1alpha1.Watcher,
	virtualService *istioclientapi.VirtualService,
) error {
	gateways, err := c.LookupGateways(ctx, watcher)
	if err != nil {
		return err
	}
	hosts, err := getHosts(gateways)
	if err != nil {
		return err
	}
	// lookup cr config
	updateHTTPRouteRequired := updateHTTPRoute(watcher, virtualService)
	updateGatewayRequired := updateGateway(gateways, virtualService)
	updateHostsRequired := updateHosts(hosts, virtualService)

	if updateHTTPRouteRequired || updateGatewayRequired || updateHostsRequired {
		return c.updateVirtualService(ctx, virtualService)
	}
	return nil
}

func updateHTTPRoute(watcher *v1alpha1.Watcher, virtualService *istioclientapi.VirtualService) bool {
	// lookup cr config
	routeIdx := lookupHTTPRouteByObjectKey(virtualService.Spec.Http, client.ObjectKeyFromObject(watcher))
	if routeIdx != -1 {
		istioHTTPRoute := prepareIstioHTTPRouteForCR(watcher)
		if isRouteConfigEqual(virtualService.Spec.Http[routeIdx], istioHTTPRoute) {
			return false
		}
		virtualService.Spec.Http[routeIdx] = prepareIstioHTTPRouteForCR(watcher)
		return true
	}
	// if route doesn't exist already append it to the route list
	istioHTTPRoute := prepareIstioHTTPRouteForCR(watcher)
	virtualService.Spec.Http = append(virtualService.Spec.Http, istioHTTPRoute)
	return true
}

func updateGateway(
	gateways []*istioclientapi.Gateway,
	virtualService *istioclientapi.VirtualService,
) bool {
	if contentDiffers(convertToGatewayList(gateways), virtualService.Spec.Gateways) {
		addGateways(gateways, virtualService)
		return true
	}
	return false
}

func updateHosts(hosts []string, virtualService *istioclientapi.VirtualService) bool {
	if contentDiffers(hosts, virtualService.Spec.Gateways) {
		virtualService.Spec.Hosts = hosts
		return true
	}
	return false
}

func contentDiffers(target []string, source []string) bool {
	if len(source) != len(target) {
		return true
	}

	targetMap := make(map[string]bool)
	for i := range target {
		targetMap[target[i]] = false
	}
	for i := range source {
		_, exists := targetMap[source[i]]
		if exists {
			targetMap[source[i]] = true
		} else {
			// source item not in target
			return true
		}
	}
	// check not matched target item
	for _, exists := range targetMap {
		if !exists {
			return true
		}
	}

	return false
}

func (c *Client) RemoveVirtualServiceConfigForCR(ctx context.Context, watcherObjKey client.ObjectKey,
) error {
	virtualService, err := c.GetVirtualService(ctx)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
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

// SyncCertificateSecretToIstio fetches the CA Root Certificate, configured by KLM flags, and creates, updates
// or deletes a copy of it in the Istio namespace. This is needed since Istio needs the Root CA Certificate secret
// in its own namespace to use it for Gateways with strict mTLS connection.
func (c *Client) SyncCertificateSecretToIstio(ctx context.Context, kcpClient client.Client) error {
	certSecret := &corev1.Secret{}
	err := kcpClient.Get(ctx,
		client.ObjectKey{
			Namespace: c.config.WatcherRootCertificateNamespace,
			Name:      c.config.WatcherRootCertificateName,
		}, certSecret)

	if apierrors.IsNotFound(err) {
		// if CA Certificate does not exist, check if it exists in istio Namespace, if yes remove it
		c.logger.V(log.DebugLevel).Info("CA Root Certificate does not exist, " +
			"will delete CA Root Certificate in istio namespace if exists")
		if err := kcpClient.Get(ctx, client.ObjectKey{
			Namespace: c.config.IstioCertificateNamespace,
			Name:      c.config.WatcherRootCertificateName,
		}, certSecret); apierrors.IsNotFound(err) {
			return nil
		} else if err != nil {
			return err
		}
		if err := kcpClient.Delete(ctx, certSecret); err != nil {
			return err
		}

		// CA Certificate has been removed in all namespaces, sync not needed
		return nil
	} else if err != nil {
		return err
	}

	istioCertSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        certSecret.Name,
			Namespace:   c.config.IstioCertificateNamespace,
			Labels:      certSecret.Labels,
			Annotations: certSecret.Annotations,
		},

		Data: certSecret.Data,
	}
	// CA Certificate exists, copy it to istio namespace

	if err := kcpClient.Update(ctx, istioCertSecret); apierrors.IsNotFound(err) {
		certSecret.ResourceVersion = ""
		if err := kcpClient.Create(ctx, istioCertSecret); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}
