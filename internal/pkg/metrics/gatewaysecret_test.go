package metrics_test

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
)

func TestGatewaySecretMetrics_ServerCertificateCloseToExpiry_WhenSet_GaugeIsOne(t *testing.T) {
	gatewaySecret := metrics.NewGatewaySecret()
	t.Cleanup(func() { ctrlmetrics.Registry.Unregister(gatewaySecret.ServerCertCloseToExpiryGauge) })

	gatewaySecret.ServerCertificateCloseToExpiry(true)

	err := testutil.CollectAndCompare(gatewaySecret.ServerCertCloseToExpiryGauge, strings.NewReader(`
		# HELP lifecycle_mgr_gateway_secret_server_cert_close_to_expiry Indicates whether the server certificate in the gateway secret is close to expiry (1) or not (0)
		# TYPE lifecycle_mgr_gateway_secret_server_cert_close_to_expiry gauge
		lifecycle_mgr_gateway_secret_server_cert_close_to_expiry 1
	`)) //nolint:revive // prometheus text format cannot be wrapped
	require.NoError(t, err)
}

func TestGatewaySecretMetrics_ServerCertificateCloseToExpiry_WhenUnset_GaugeIsZero(t *testing.T) {
	gatewaySecret := metrics.NewGatewaySecret()
	t.Cleanup(func() { ctrlmetrics.Registry.Unregister(gatewaySecret.ServerCertCloseToExpiryGauge) })

	gatewaySecret.ServerCertificateCloseToExpiry(false)

	err := testutil.CollectAndCompare(gatewaySecret.ServerCertCloseToExpiryGauge, strings.NewReader(`
		# HELP lifecycle_mgr_gateway_secret_server_cert_close_to_expiry Indicates whether the server certificate in the gateway secret is close to expiry (1) or not (0)
		# TYPE lifecycle_mgr_gateway_secret_server_cert_close_to_expiry gauge
		lifecycle_mgr_gateway_secret_server_cert_close_to_expiry 0
	`)) //nolint:revive // prometheus text format cannot be wrapped
	require.NoError(t, err)
}

func TestGatewaySecretMetrics_ServerCertificateCloseToExpiry_WhenToggledToFalse_GaugeIsZero(t *testing.T) {
	gatewaySecret := metrics.NewGatewaySecret()
	t.Cleanup(func() { ctrlmetrics.Registry.Unregister(gatewaySecret.ServerCertCloseToExpiryGauge) })

	gatewaySecret.ServerCertificateCloseToExpiry(true)
	gatewaySecret.ServerCertificateCloseToExpiry(false)

	err := testutil.CollectAndCompare(gatewaySecret.ServerCertCloseToExpiryGauge, strings.NewReader(`
		# HELP lifecycle_mgr_gateway_secret_server_cert_close_to_expiry Indicates whether the server certificate in the gateway secret is close to expiry (1) or not (0)
		# TYPE lifecycle_mgr_gateway_secret_server_cert_close_to_expiry gauge
		lifecycle_mgr_gateway_secret_server_cert_close_to_expiry 0
	`)) //nolint:revive // prometheus text format cannot be wrapped
	require.NoError(t, err)
}
