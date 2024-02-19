package istio

import (
	"context"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	istioapiv1beta1 "istio.io/api/networking/v1beta1"
	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type (
	gatewayLookup interface {
		LookupGateways(ctx context.Context, watcher *v1beta2.Watcher) ([]*istioclientapiv1beta1.Gateway, error)
	}

	Service struct {
		GatewayLookup gatewayLookup
	}
)

func (s Service) NewVirtualServiceForWatcher(ctx context.Context, watcher *v1beta2.Watcher, targetNamespace string,
) (*istioclientapiv1beta1.VirtualService, error) {
	virtualSvc := &istioclientapiv1beta1.VirtualService{}

	if watcher == nil {
		return virtualSvc, nil
	}

	virtualSvc.SetName(watcher.Name)
	virtualSvc.SetNamespace(targetNamespace)

	gateways, err := s.GatewayLookup.LookupGateways(ctx, watcher)
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
