package watcher

import (
	"context"
	"fmt"
	"net"
	"strconv"

	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type GatewayConfig struct {
	// IstioGatewayName represents the cluster resource name of the klm istio gateway
	IstioGatewayName string
	// IstioGatewayNamespace represents the cluster resource namespace of the klm istio gateway
	IstioGatewayNamespace string
	// LocalGatewayPortOverwrite indicates the port used to expose the KCP cluster locally in k3d
	// for the watcher callbacks
	LocalGatewayPortOverwrite string
}

func (g GatewayConfig) ResolveKcpAddr(kcpClient client.Client) (string, error) { // Get public KCP DNS name and port from the Gateway

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	gateway := &istioclientapiv1beta1.Gateway{}
	if err := kcpClient.Get(ctx, client.ObjectKey{
		Namespace: g.IstioGatewayNamespace,
		Name:      g.IstioGatewayName,
	}, gateway); err != nil {
		return "", fmt.Errorf("failed to get istio gateway %s: %w", g.IstioGatewayName, err)
	}

	if len(gateway.Spec.GetServers()) != 1 || len(gateway.Spec.GetServers()[0].GetHosts()) != 1 {
		return "", ErrGatewayHostWronglyConfigured
	}
	host := gateway.Spec.GetServers()[0].GetHosts()[0]
	port := gateway.Spec.GetServers()[0].GetPort().GetNumber()

	if g.LocalGatewayPortOverwrite != "" {
		return net.JoinHostPort(host, g.LocalGatewayPortOverwrite), nil
	}

	return net.JoinHostPort(host, strconv.Itoa(int(port))), nil
}
