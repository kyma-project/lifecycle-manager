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
	gs := metrics.NewGatewaySecret()
	t.Cleanup(func() { ctrlmetrics.Registry.Unregister(gs.ServerCertCloseToExpiryGauge) })

	gs.ServerCertificateCloseToExpiry(true)

	err := testutil.CollectAndCompare(gs.ServerCertCloseToExpiryGauge, strings.NewReader(`
		# HELP lifecycle_mgr_gateway_secret_server_cert_close_to_expiry Indicates whether the server certificate in the gateway secret is close to expiry (1) or not (0)
		# TYPE lifecycle_mgr_gateway_secret_server_cert_close_to_expiry gauge
		lifecycle_mgr_gateway_secret_server_cert_close_to_expiry 1
	`))
	require.NoError(t, err)
}

func TestGatewaySecretMetrics_ServerCertificateCloseToExpiry_WhenUnset_GaugeIsZero(t *testing.T) {
	gs := metrics.NewGatewaySecret()
	t.Cleanup(func() { ctrlmetrics.Registry.Unregister(gs.ServerCertCloseToExpiryGauge) })

	gs.ServerCertificateCloseToExpiry(false)

	err := testutil.CollectAndCompare(gs.ServerCertCloseToExpiryGauge, strings.NewReader(`
		# HELP lifecycle_mgr_gateway_secret_server_cert_close_to_expiry Indicates whether the server certificate in the gateway secret is close to expiry (1) or not (0)
		# TYPE lifecycle_mgr_gateway_secret_server_cert_close_to_expiry gauge
		lifecycle_mgr_gateway_secret_server_cert_close_to_expiry 0
	`))
	require.NoError(t, err)
}

func TestGatewaySecretMetrics_ServerCertificateCloseToExpiry_WhenToggledToFalse_GaugeIsZero(t *testing.T) {
	gs := metrics.NewGatewaySecret()
	t.Cleanup(func() { ctrlmetrics.Registry.Unregister(gs.ServerCertCloseToExpiryGauge) })

	gs.ServerCertificateCloseToExpiry(true)
	gs.ServerCertificateCloseToExpiry(false)

	err := testutil.CollectAndCompare(gs.ServerCertCloseToExpiryGauge, strings.NewReader(`
		# HELP lifecycle_mgr_gateway_secret_server_cert_close_to_expiry Indicates whether the server certificate in the gateway secret is close to expiry (1) or not (0)
		# TYPE lifecycle_mgr_gateway_secret_server_cert_close_to_expiry gauge
		lifecycle_mgr_gateway_secret_server_cert_close_to_expiry 0
	`))
	require.NoError(t, err)
}
