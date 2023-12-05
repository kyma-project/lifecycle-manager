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

func resolveKcpAddr(ctx context.Context, kcpClient client.Client,
	gatewayConfig GatewayConfig,
) (string, error) { // Get public KCP DNS name and port from the Gateway
	gateway := &istioclientapiv1beta1.Gateway{}

	if err := kcpClient.Get(ctx, client.ObjectKey{
		Namespace: gatewayConfig.IstioGatewayNamespace,
		Name:      gatewayConfig.IstioGatewayName,
	}, gateway); err != nil {
		return "", fmt.Errorf("failed to get istio gateway %s: %w", gatewayConfig.IstioGatewayName, err)
	}

	if len(gateway.Spec.GetServers()) != 1 || len(gateway.Spec.GetServers()[0].GetHosts()) != 1 {
		return "", ErrGatewayHostWronglyConfigured
	}
	host := gateway.Spec.GetServers()[0].GetHosts()[0]
	port := gateway.Spec.GetServers()[0].GetPort().GetNumber()

	if gatewayConfig.LocalGatewayPortOverwrite != "" {
		return net.JoinHostPort(host, gatewayConfig.LocalGatewayPortOverwrite), nil
	}

	return net.JoinHostPort(host, strconv.Itoa(int(port))), nil
}
