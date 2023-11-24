package metrics

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

const (
	metricPurgeTime                     = "lifecycle_mgr_purgectrl_time"
	metricPurgeRequests                 = "lifecycle_mgr_purgectrl_requests_total"
	metricPurgeError                    = "lifecycle_mgr_purgectrl_error"
	errorReasonLabel                    = "err_reason"
	ErrPurgeFinalizerRemoval PurgeError = "PurgeFinalizerRemovalError"
	ErrCleanup               PurgeError = "CleanupError"
)

type PurgeError string

type PurgeMetrics struct {
	purgeTimeGauge       prometheus.Gauge
	purgeRequestsCounter prometheus.Counter
	purgeErrorGauge      *prometheus.GaugeVec
}

func NewPurgeMetrics() *PurgeMetrics {
	purgeMetrics := &PurgeMetrics{
		purgeTimeGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: metricPurgeTime,
			Help: "Indicates average purge duration",
		}),
		purgeRequestsCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Name: metricPurgeRequests,
			Help: "Indicates total purge count ",
		}),
		purgeErrorGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: metricPurgeError,
			Help: "Indicates purge errors",
		}, []string{kymaNameLabel, shootIDLabel, instanceIDLabel, errorReasonLabel}),
	}
	ctrlmetrics.Registry.MustRegister(purgeMetrics.purgeTimeGauge)
	ctrlmetrics.Registry.MustRegister(purgeMetrics.purgeRequestsCounter)
	ctrlmetrics.Registry.MustRegister(purgeMetrics.purgeErrorGauge)
	return purgeMetrics
}

func (p *PurgeMetrics) UpdatePurgeCount() {
	p.purgeRequestsCounter.Inc()
}

func (p *PurgeMetrics) UpdatePurgeTime(duration time.Duration) {
	p.purgeTimeGauge.Set(duration.Seconds())
}

func (p *PurgeMetrics) UpdatePurgeError(kyma *v1beta2.Kyma, purgeError PurgeError) error {
	shootID, err := ExtractShootID(kyma)
	if err != nil {
		return fmt.Errorf("%w: %w", errMetric, err)
	}
	instanceID, err := ExtractInstanceID(kyma)
	if err != nil {
		return fmt.Errorf("%w: %w", errMetric, err)
	}
	metric, err := p.purgeErrorGauge.GetMetricWith(prometheus.Labels{
		kymaNameLabel:    kyma.Name,
		shootIDLabel:     shootID,
		instanceIDLabel:  instanceID,
		errorReasonLabel: string(purgeError),
	})
	if err != nil {
		return fmt.Errorf("%w: %w", errMetric, err)
	}
	metric.Set(1)

	return nil
}
