package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

type SharedMetrics struct {
	requeueReasonCounter *prometheus.CounterVec
}

func NewSharedMetrics() *SharedMetrics {
	metrics := &SharedMetrics{
		requeueReasonCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: MetricRequeueReason,
			Help: "Indicates the reason for requeue",
		}, []string{requeueReasonLabel, requeueTypeLabel}),
	}
	ctrlmetrics.Registry.MustRegister(metrics.requeueReasonCounter)
	return metrics
}
