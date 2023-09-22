package metrics

import (
	"errors"
	"fmt"

	ctrlMetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/controller/common/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricPurgeTime     = "lifecycle_mgr_kyma_state"
	metricPurgeRequests = "lifecycle_mgr_module_state"
	kymaNameLabel       = "kyma_name"
	shootIDLabel        = "shoot"
	instanceIDLabel     = "instance_id"
)

var (
	metricPurgeTimeGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{ //nolint:gochecknoglobals
		Name: metricPurgeTime,
		Help: "Indicates ",
	}, []string{kymaNameLabel, shootIDLabel, instanceIDLabel})
	metricPurgeRequestsGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{ //nolint:gochecknoglobals
		Name: metricPurgeRequests,
		Help: "Indicates ",
	}, []string{kymaNameLabel, shootIDLabel, instanceIDLabel})
)

func Initialize() {
	ctrlMetrics.Registry.MustRegister(metricPurgeTimeGauge)
	ctrlMetrics.Registry.MustRegister(metricPurgeRequestsGauge)
}

var errMetric = errors.New("failed to update metrics")

// UpdateAll sets all purge-controller related metrics.
func UpdateAll(kyma *v1beta2.Kyma) error {
	shootID, err := metrics.ExtractShootID(kyma)
	if err != nil {
		return fmt.Errorf("%w: %w", errMetric, err)
	}
	instanceID, err := metrics.ExtractInstanceID(kyma)
	if err != nil {
		return fmt.Errorf("%w: %w", errMetric, err)
	}
	// TODO set metrics

	return nil
}
