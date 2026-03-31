package metrics_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
)

func expectedGatewaySecretMetricOutput(value int) string {
	lines := []string{
		"# HELP " + metrics.MetricGatewaySecretServerCertCloseToExpiry + " " +
			metrics.MetricHelpGatewaySecretServerCertCloseToExpiry,
		"# TYPE " + metrics.MetricGatewaySecretServerCertCloseToExpiry + " gauge",
		fmt.Sprintf("%s %d", metrics.MetricGatewaySecretServerCertCloseToExpiry, value),
	}
	return strings.Join(lines, "\n") + "\n"
}

func TestGatewaySecretMetrics_ServerCertificateCloseToExpiry_WhenSet_GaugeIsOne(t *testing.T) {
	gatewaySecret := metrics.NewGatewaySecret()
	t.Cleanup(func() { ctrlmetrics.Registry.Unregister(gatewaySecret.ServerCertCloseToExpiryGauge) })

	gatewaySecret.ServerCertificateCloseToExpiry(true)

	err := testutil.CollectAndCompare(gatewaySecret.ServerCertCloseToExpiryGauge,
		strings.NewReader(expectedGatewaySecretMetricOutput(1)))
	require.NoError(t, err)
}

func TestGatewaySecretMetrics_ServerCertificateCloseToExpiry_WhenUnset_GaugeIsZero(t *testing.T) {
	gatewaySecret := metrics.NewGatewaySecret()
	t.Cleanup(func() { ctrlmetrics.Registry.Unregister(gatewaySecret.ServerCertCloseToExpiryGauge) })

	gatewaySecret.ServerCertificateCloseToExpiry(false)

	err := testutil.CollectAndCompare(gatewaySecret.ServerCertCloseToExpiryGauge,
		strings.NewReader(expectedGatewaySecretMetricOutput(0)))
	require.NoError(t, err)
}

func TestGatewaySecretMetrics_ServerCertificateCloseToExpiry_WhenToggledToFalse_GaugeIsZero(t *testing.T) {
	gatewaySecret := metrics.NewGatewaySecret()
	t.Cleanup(func() { ctrlmetrics.Registry.Unregister(gatewaySecret.ServerCertCloseToExpiryGauge) })

	gatewaySecret.ServerCertificateCloseToExpiry(true)
	gatewaySecret.ServerCertificateCloseToExpiry(false)

	err := testutil.CollectAndCompare(gatewaySecret.ServerCertCloseToExpiryGauge,
		strings.NewReader(expectedGatewaySecretMetricOutput(0)))
	require.NoError(t, err)
}
