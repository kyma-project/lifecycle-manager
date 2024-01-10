package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

const (
	MetricPurgeTime                     = "lifecycle_mgr_purgectrl_time"
	MetricPurgeRequests                 = "lifecycle_mgr_purgectrl_requests_total"
	MetricPurgeError                    = "lifecycle_mgr_purgectrl_error"
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
			Name: MetricPurgeTime,
			Help: "Indicates average purge duration",
		}),
		purgeRequestsCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Name: MetricPurgeRequests,
			Help: "Indicates total purge count ",
		}),
		purgeErrorGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: MetricPurgeError,
			Help: "Indicates purge errors",
		}, []string{KymaNameLabel, shootIDLabel, instanceIDLabel, errorReasonLabel}),
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

func (p *PurgeMetrics) UpdatePurgeError(ctx context.Context, kyma *v1beta2.Kyma, purgeError PurgeError) {
	shootID, err := ExtractShootID(kyma)
	if err != nil {
		logf.FromContext(ctx).Error(err, "Failed to update error metrics")
		return
	}
	instanceID, err := ExtractInstanceID(kyma)
	if err != nil {
		return
	}
	metric, err := p.purgeErrorGauge.GetMetricWith(prometheus.Labels{
		KymaNameLabel:    kyma.Name,
		shootIDLabel:     shootID,
		instanceIDLabel:  instanceID,
		errorReasonLabel: string(purgeError),
	})
	if err != nil {
		logf.FromContext(ctx).Error(err, "Failed to update error metrics")
		return
	}
	metric.Set(1)
}
