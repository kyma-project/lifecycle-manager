package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	//nolint:gosec // not a credential
	MetricGatewaySecretServerCertCloseToExpiry     = "lifecycle_mgr_gateway_secret_server_cert_close_to_expiry"
	MetricHelpGatewaySecretServerCertCloseToExpiry = "Indicates whether the server certificate in the gateway secret" +
		" is close to expiry (1) or not (0)"
)

type GatewaySecret struct {
	ServerCertCloseToExpiryGauge prometheus.Gauge
}

func NewGatewaySecret() *GatewaySecret {
	serverCertCloseToExpiryGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: MetricGatewaySecretServerCertCloseToExpiry,
		Help: MetricHelpGatewaySecretServerCertCloseToExpiry,
	})
	ctrlmetrics.Registry.MustRegister(serverCertCloseToExpiryGauge)

	return &GatewaySecret{
		ServerCertCloseToExpiryGauge: serverCertCloseToExpiryGauge,
	}
}

func (gs *GatewaySecret) ServerCertificateCloseToExpiry(set bool) {
	if set {
		gs.ServerCertCloseToExpiryGauge.Set(1)
	} else {
		gs.ServerCertCloseToExpiryGauge.Set(0)
	}
}
