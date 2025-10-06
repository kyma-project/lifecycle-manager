package istio

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type Client struct {
	istioclient.Interface

	logger logr.Logger
}

func NewIstioClient(cfg *rest.Config, logger logr.Logger) (*Client, error) {
	cs, err := istioclient.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Join(ErrFailedToCreateIstioClient, err)
	}
	return &Client{
		Interface: cs,
		logger:    logger,
	}, nil
}

func (c *Client) GetVirtualService(ctx context.Context, name, namespace string) (*istioclientapiv1beta1.VirtualService,
	error,
) {
	virtualService, err := c.NetworkingV1beta1().
		VirtualServices(namespace).
		Get(ctx, name, apimetav1.GetOptions{})
	if err != nil {
		return nil, errors.Join(ErrFailedToGetVirtualService, err)
	}
	return virtualService, nil
}

func (c *Client) ListVirtualServices(ctx context.Context, namespace string) (*istioclientapiv1beta1.VirtualServiceList,
	error,
) {
	virtualServiceList, err := c.NetworkingV1beta1().
		VirtualServices(namespace).
		List(ctx, apimetav1.ListOptions{})
	if err != nil {
		return nil, errors.Join(ErrFailedToListVirtualServices, err)
	}
	return virtualServiceList, nil
}

func (c *Client) CreateVirtualService(ctx context.Context, virtualService *istioclientapiv1beta1.VirtualService) error {
	_, err := c.NetworkingV1beta1().
		VirtualServices(virtualService.GetNamespace()).
		Create(ctx, virtualService, apimetav1.CreateOptions{})
	if err != nil {
		return errors.Join(ErrFailedToCreateVirtualService, err)
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
		return errors.Join(ErrFailedToUpdateVirtualService, err)
	}
	return nil
}

func (c *Client) DeleteVirtualService(ctx context.Context, name, namespace string,
) error {
	err := c.NetworkingV1beta1().
		VirtualServices(namespace).
		Delete(ctx, name, apimetav1.DeleteOptions{})
	if err != nil {
		return errors.Join(ErrFailedToDeleteVirtualService, err)
	}
	return nil
}

func (c *Client) ListGatewaysByLabelSelector(ctx context.Context, labelSelector *apimetav1.LabelSelector,
	gatewayNamespace string) (*istioclientapiv1beta1.GatewayList,
	error,
) {
	selector, err := apimetav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return nil, errors.Join(ErrFailedToConvertLabelSelector, err)
	}

	selectorString := selector.String()
	gateways, err := c.NetworkingV1beta1().
		Gateways(gatewayNamespace).
		List(ctx, apimetav1.ListOptions{
			LabelSelector: selectorString,
		})
	if err != nil {
		return nil, errors.Join(fmt.Errorf("%w, %q", ErrFailedToGetGatewayByLabelSelector, selectorString), err)
	}

	return gateways, nil
}
