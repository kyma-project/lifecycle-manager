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

func (c *Client) GetVirtualService(ctx context.Context, vsName, vsNamespace string) (*istioclientapiv1beta1.VirtualService, error) {
	virtualService, err := c.NetworkingV1beta1().
		VirtualServices(vsNamespace).
		Get(ctx, vsName, apimetav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch virtual service %w", err)
	}
	return virtualService, nil
}

func (c *Client) ListVirtualServices(ctx context.Context, namespace string) (*istioclientapiv1beta1.VirtualServiceList, error) {
	virtualServiceList, err := c.NetworkingV1beta1().
		VirtualServices(namespace).
		List(ctx, apimetav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list virtual services %w", err)
	}
	return virtualServiceList, nil
}

func (c *Client) CreateVirtualService(ctx context.Context, virtualSvc *istioclientapiv1beta1.VirtualService) error {
	_, err := c.NetworkingV1beta1().
		VirtualServices(virtualSvc.GetNamespace()).
		Create(ctx, virtualSvc, apimetav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create istio virtual service: %w", err)
	}
	return nil
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

func (c *Client) RemoveVirtualServiceForCR(ctx context.Context, watcherObjKey client.ObjectKey, vsNamespace string,
) error {
	err := c.NetworkingV1beta1().
		VirtualServices(vsNamespace).
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
