package gateway

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"

	skrwebhookresources "github.com/kyma-project/lifecycle-manager/internal/service/watcher/resources"
)

var (
	ErrGatewayHostWronglyConfigured = errors.New("gateway should have configured exactly one server and one host")
	ErrNoHostnameInGateway          = errors.New("the gateway has no host specified")
)

type IstioGatewayRepository interface {
	Get(ctx context.Context, name, namespace string) (*istioclientapiv1beta1.Gateway, error)
}

type Service struct {
	istioGatewayRepository IstioGatewayRepository
	// istioGatewayName represents the cluster resource name of the klm istio gateway
	istioGatewayName string
	// istioGatewayNamespace represents the cluster resource namespace of the klm istio gateway
	istioGatewayNamespace string
	// localGatewayPortOverwrite indicates the port used to expose the KCP cluster locally in k3d
	// for the watcher callbacks
	localGatewayPortOverwrite string
}

func NewService(
	istioGatewayName, istioGatewayNamespace, localGatewayPortOverwrite string,
	gatewayRepository IstioGatewayRepository,
) *Service {
	return &Service{
		istioGatewayName:          istioGatewayName,
		istioGatewayNamespace:     istioGatewayNamespace,
		localGatewayPortOverwrite: localGatewayPortOverwrite,
		istioGatewayRepository:    gatewayRepository,
	}
}

func (s *Service) ResolveKcpAddr() (*skrwebhookresources.KCPAddr,
	error,
) { // Get public KCP DNS name and port from the Gateway
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gateway, err := s.istioGatewayRepository.Get(ctx, s.istioGatewayName, s.istioGatewayNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get istio gateway %s: %w", s.istioGatewayName, err)
	}

	if len(gateway.Spec.GetServers()) != 1 || len(gateway.Spec.GetServers()[0].GetHosts()) != 1 {
		return nil, ErrGatewayHostWronglyConfigured
	}

	kcpAddr := &skrwebhookresources.KCPAddr{
		Hostname: gateway.Spec.GetServers()[0].GetHosts()[0],
		Port:     gateway.Spec.GetServers()[0].GetPort().GetNumber(),
	}

	if len(strings.TrimSpace(kcpAddr.Hostname)) == 0 {
		return nil, ErrNoHostnameInGateway
	}

	if s.localGatewayPortOverwrite != "" {
		port, err := strconv.ParseInt(s.localGatewayPortOverwrite, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid gateway port specified %s, must be a number (%w)",
				s.localGatewayPortOverwrite, err)
		}
		kcpAddr.Port = uint32(port) //nolint:gosec // G115: this is not a security sensitive code, just a port number
	}
	return kcpAddr, nil
}
