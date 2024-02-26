package istio

import (
	"errors"
	"fmt"

	istioapiv1beta1 "istio.io/api/networking/v1beta1"
	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

const (
	contractVersion = "v1"
	prefixFormat    = "/%s/%s/event"
)

func NewVirtualService(namespace string, watcher *v1beta2.Watcher, gateways *istioclientapiv1beta1.GatewayList) (*istioclientapiv1beta1.VirtualService, error) {
	if namespace == "" {
		return nil, fmt.Errorf("%w namespace", ErrInvalidArgument)
	}

	if watcher == nil {
		return nil, fmt.Errorf("%w watcher", ErrInvalidArgument)
	}

	if watcher.Name == "" {
		return nil, fmt.Errorf("%w watcher.Name", ErrInvalidArgument)
	}

	if gateways == nil {
		return nil, fmt.Errorf("%w gateways", ErrInvalidArgument)
	}

	if len(gateways.Items) == 0 {
		return nil, fmt.Errorf("%w gateways.Items", ErrInvalidArgument)
	}

	hosts, err := getHosts(gateways.Items)
	if err != nil {
		return nil, errors.Join(ErrInvalidArgument, err)
	}

	virtualService := &istioclientapiv1beta1.VirtualService{}
	virtualService.SetName(watcher.Name)
	virtualService.SetNamespace(namespace)
	virtualService.Spec.Gateways = getGatewayNames(gateways.Items)
	virtualService.Spec.Hosts = hosts
	virtualService.Spec.Http = []*istioapiv1beta1.HTTPRoute{
		PrepareIstioHTTPRouteForCR(watcher),
	}

	return virtualService, nil
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

func getGatewayNames(gateways []*istioclientapiv1beta1.Gateway) []string {
	gatewayNames := make([]string, 0)
	for _, gateway := range gateways {
		gatewayNames = append(gatewayNames, client.ObjectKeyFromObject(gateway).String())
	}
	return gatewayNames
}

func getHosts(gateways []*istioclientapiv1beta1.Gateway) ([]string, error) {
	hosts := make([]string, 0)

	for _, gateway := range gateways {
		gatewayHosts := make([]string, 0)
		for _, server := range gateway.Spec.GetServers() {
			gatewayHosts = append(gatewayHosts, server.GetHosts()...)
		}

		if len(gatewayHosts) == 0 {
			return nil, fmt.Errorf("for gateway %s: %w",
				client.ObjectKeyFromObject(gateway).String(),
				ErrCantFindGatewayServersHost)
		}

		hosts = append(hosts, gatewayHosts...)
	}

	return hosts, nil
}

func destinationHost(serviceName, serviceNamespace string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, serviceNamespace)
}
