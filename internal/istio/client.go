package istio

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	istioapiv1beta1 "istio.io/api/networking/v1beta1"
	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
)

const (
	firstElementIdx = 0
)

type Client struct {
	istioclient.Interface
	eventRecorder record.EventRecorder
	logger        logr.Logger
}

func NewIstioClient(cfg *rest.Config, recorder record.EventRecorder,
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

func (c *Client) GetVirtualService(ctx context.Context, name, namespace string) (*istioclientapiv1beta1.VirtualService, error) {
	virtualService, err := c.NetworkingV1beta1().
		VirtualServices(namespace).
		Get(ctx, name, apimetav1.GetOptions{})
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

func (c *Client) CreateVirtualService(ctx context.Context, virtualService *istioclientapiv1beta1.VirtualService) error {
	_, err := c.NetworkingV1beta1().
		VirtualServices(virtualService.GetNamespace()).
		Create(ctx, virtualService, apimetav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create istio virtual service: %w", err)
	}
	return nil
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

func (c *Client) DeleteVirtualService(ctx context.Context, name, namespace string,
) error {
	err := c.NetworkingV1beta1().
		VirtualServices(namespace).
		Delete(ctx, name, apimetav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete virtual service for cr: %w", err)
	}
	return nil
}

func (c *Client) ListGatewaysByLabelSelector(ctx context.Context, labelSelector *apimetav1.LabelSelector) (*istioclientapiv1beta1.GatewayList,
	error,
) {
	selector, err := apimetav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return nil, fmt.Errorf("error converting label selector: %w", err)
	}

	selectorString := selector.String()
	gateways, err := c.NetworkingV1beta1().
		Gateways(apimetav1.NamespaceAll).
		List(ctx, apimetav1.ListOptions{
			LabelSelector: selectorString,
		})
	if err != nil {
		return nil, fmt.Errorf("error looking up Istio gateway with the label selector %q: %w",
			selectorString, err)
	}

	return gateways, nil
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
