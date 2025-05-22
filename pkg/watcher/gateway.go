package watcher

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var errNoHostnameInGateway = errors.New("the gateway has no host specified")

type GatewayConfig struct {
	// IstioGatewayName represents the cluster resource name of the klm istio gateway
	IstioGatewayName string
	// IstioGatewayNamespace represents the cluster resource namespace of the klm istio gateway
	IstioGatewayNamespace string
	// LocalGatewayPortOverwrite indicates the port used to expose the KCP cluster locally in k3d
	// for the watcher callbacks
	LocalGatewayPortOverwrite string
}

func (g GatewayConfig) ResolveKcpAddr(mgr ctrl.Manager) (*KCPAddr, error) { // Get public KCP DNS name and port from the Gateway
	kcpClient, err := client.New(mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme()})
	if err != nil {
		return nil, fmt.Errorf("can't create kcpClient: %w", err)
	}

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

	var kcpAddr KCPAddr
	kcpAddr.Hostname = gateway.Spec.GetServers()[0].GetHosts()[0]
	if len(strings.TrimSpace(kcpAddr.Hostname)) == 0 {
		return nil, errNoHostnameInGateway
	}
	if g.LocalGatewayPortOverwrite != "" {
		kcpAddr.Port, err = strconv.Atoi(g.LocalGatewayPortOverwrite)
		if err != nil {
			return nil, fmt.Errorf("invalid gateway port specified %s, must be a number (%w)", g.LocalGatewayPortOverwrite, err)
		}
	} else {
		kcpAddr.Port = int(gateway.Spec.GetServers()[0].GetPort().GetNumber())
	}

	return &kcpAddr, nil
}
