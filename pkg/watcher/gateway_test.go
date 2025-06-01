package watcher_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"istio.io/api/networking/v1alpha3"
	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

const (
	namespace = "test-namespace"
	name      = "test-gw"
	host      = "example.com"
	port      = 8443
)

func TestResolveKcpAddr(t *testing.T) {
	t.Run("success with default port", func(t *testing.T) {
		gw := newGateway([]string{host}, port)
		cl := buildClient(t, gw)
		cfg := watcher.GatewayConfig{IstioGatewayName: name, IstioGatewayNamespace: namespace}
		kcpAddr, err := cfg.ResolveKcpAddr(cl)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if kcpAddr.Hostname != host || kcpAddr.Port != port {
			t.Errorf("unexpected kcpAddr: %+v", kcpAddr)
		}
	})

	t.Run("success with port overwrite", func(t *testing.T) {
		portToOverwrite := "12345"
		gw := newGateway([]string{host}, port)
		clnt := buildClient(t, gw)
		cfg := watcher.GatewayConfig{
			IstioGatewayName:          name,
			IstioGatewayNamespace:     namespace,
			LocalGatewayPortOverwrite: portToOverwrite,
		}
		addr, err := cfg.ResolveKcpAddr(clnt)
		require.NoError(t, err)
		expectedPort, err := strconv.Atoi(portToOverwrite)
		require.NoError(t, err)
		require.Equal(t, expectedPort, int(addr.Port))
	})

	t.Run("error when host is empty", func(t *testing.T) {
		gw := newGateway([]string{""}, port)
		cl := buildClient(t, gw)
		cfg := watcher.GatewayConfig{IstioGatewayName: name, IstioGatewayNamespace: namespace}
		_, err := cfg.ResolveKcpAddr(cl)
		if !errors.Is(err, watcher.ErrNoHostnameInGateway) {
			t.Errorf("expected ErrNoHostnameInGateway, got %v", err)
		}
	})

	t.Run("error when gateway not found", func(t *testing.T) {
		cl := fake.NewClientBuilder().Build()
		cfg := watcher.GatewayConfig{IstioGatewayName: name, IstioGatewayNamespace: namespace}
		_, err := cfg.ResolveKcpAddr(cl)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("error when multiple hosts", func(t *testing.T) {
		gw := newGateway([]string{host, "other.com"}, port)
		cl := buildClient(t, gw)
		cfg := watcher.GatewayConfig{IstioGatewayName: name, IstioGatewayNamespace: namespace}
		_, err := cfg.ResolveKcpAddr(cl)
		if !errors.Is(err, watcher.ErrGatewayHostWronglyConfigured) {
			t.Errorf("expected ErrGatewayHostWronglyConfigured, got %v", err)
		}
	})
}

//nolint:ireturn // buildClient creates a fake client with the provided Gateway object.
func buildClient(t *testing.T, gw *istioclientapiv1beta1.Gateway) client.WithWatch {
	t.Helper()
	scheme := machineryruntime.NewScheme()
	err := istioclientapiv1beta1.AddToScheme(scheme)
	require.NoError(t, err)
	return fake.NewClientBuilder().WithObjects(gw).WithScheme(scheme).Build()
}

//nolint:unparam // newGateway creates a new Gateway object with the specified hosts and port number.
func newGateway(hosts []string, portNum uint32) *istioclientapiv1beta1.Gateway {
	return &istioclientapiv1beta1.Gateway{
		ObjectMeta: apimetav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
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
