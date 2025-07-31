package gateway_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/gateway"
	skrwebhookresources "github.com/kyma-project/lifecycle-manager/internal/service/watcher/resources"

	"github.com/stretchr/testify/require"
	"istio.io/api/networking/v1alpha3"
	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
)

type fakeGatewayRepo struct {
	gw  *istioclientapiv1beta1.Gateway
	err error
}

func (f *fakeGatewayRepo) Get(_ context.Context, _, _ string) (*istioclientapiv1beta1.Gateway, error) {
	return f.gw, f.err
}

//nolint:unparam // portNum is configurable
func newGateway(hosts []string, portNum uint32) *istioclientapiv1beta1.Gateway {
	return &istioclientapiv1beta1.Gateway{
		Spec: v1alpha3.Gateway{
			Servers: []*v1alpha3.Server{
				{
					Hosts: hosts,
					Port: &v1alpha3.Port{
						Number: portNum,
					},
				},
			},
		},
	}
}

func TestResolveKcpAddr(t *testing.T) {
	const (
		name      = "test-gw"
		namespace = "test-ns"
		host      = "example.com"
		port      = 8443
	)

	t.Run("success with default port", func(t *testing.T) {
		repo := &fakeGatewayRepo{gw: newGateway([]string{host}, port)}
		svc := gateway.NewService(name, namespace, "", repo)
		addr, err := svc.ResolveKcpAddr()
		require.NoError(t, err)
		require.Equal(t, &skrwebhookresources.KCPAddr{Hostname: host, Port: port}, addr)
	})

	t.Run("success with port overwrite", func(t *testing.T) {
		repo := &fakeGatewayRepo{gw: newGateway([]string{host}, port)}
		svc := gateway.NewService(name, namespace, "12345", repo)
		addr, err := svc.ResolveKcpAddr()
		require.NoError(t, err)
		require.Equal(t, uint32(12345), addr.Port)
		require.Equal(t, host, addr.Hostname)
	})

	t.Run("error when host is empty", func(t *testing.T) {
		repo := &fakeGatewayRepo{gw: newGateway([]string{""}, port)}
		svc := gateway.NewService(name, namespace, "", repo)
		_, err := svc.ResolveKcpAddr()
		require.ErrorIs(t, err, gateway.ErrNoHostnameInGateway)
	})

	t.Run("error when gateway not found", func(t *testing.T) {
		repo := &fakeGatewayRepo{err: errors.New("not found")}
		svc := gateway.NewService(name, namespace, "", repo)
		_, err := svc.ResolveKcpAddr()
		require.Error(t, err)
	})

	t.Run("error when multiple hosts", func(t *testing.T) {
		repo := &fakeGatewayRepo{gw: newGateway([]string{host, "other.com"}, port)}
		svc := gateway.NewService(name, namespace, "", repo)
		_, err := svc.ResolveKcpAddr()
		require.ErrorIs(t, err, gateway.ErrGatewayHostWronglyConfigured)
	})
}
