package watcher

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	skrwebhookresources "github.com/kyma-project/lifecycle-manager/pkg/watcher/skr_webhook_resources"
)

var (
	ErrGatewayHostWronglyConfigured = errors.New("gateway should have configured exactly one server and one host")
	ErrNoHostnameInGateway          = errors.New("the gateway has no host specified")
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

func (g GatewayConfig) ResolveKcpAddr(kcpClient client.Client) (*skrwebhookresources.KCPAddr,
	error,
) { // Get public KCP DNS name and port from the Gateway
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	gateway := &istioclientapiv1beta1.Gateway{}
	if err := kcpClient.Get(ctx, client.ObjectKey{
		Namespace: g.IstioGatewayNamespace,
		Name:      g.IstioGatewayName,
	}, gateway); err != nil {
		return nil, fmt.Errorf("failed to get istio gateway %s: %w", g.IstioGatewayName, err)
	}

	if len(gateway.Spec.GetServers()) != 1 || len(gateway.Spec.GetServers()[0].GetHosts()) != 1 {
		return nil, ErrGatewayHostWronglyConfigured
	}

	var kcpAddr skrwebhookresources.KCPAddr
	kcpAddr.Hostname = gateway.Spec.GetServers()[0].GetHosts()[0]
	if len(strings.TrimSpace(kcpAddr.Hostname)) == 0 {
		return nil, ErrNoHostnameInGateway
	}
	if g.LocalGatewayPortOverwrite != "" {
		port, err := strconv.ParseInt(g.LocalGatewayPortOverwrite, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid gateway port specified %s, must be a number (%w)",
				g.LocalGatewayPortOverwrite, err)
		}
		kcpAddr.Port = uint32(port) //nolint:gosec // G115: this is not a security sensitive code, just a port number
	} else {
		kcpAddr.Port = gateway.Spec.GetServers()[0].GetPort().GetNumber()
	}

	return &kcpAddr, nil
}
