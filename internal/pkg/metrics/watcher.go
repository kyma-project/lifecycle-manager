package metrics

import (
	watchermetrics "github.com/kyma-project/runtime-watcher/listener/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	CertNotRenewMetrics = "lifecycle_mgr_cert_not_renew"
)

type WatcherMetrics struct {
	certNotRenewGauge *prometheus.GaugeVec
}

func NewWatcherMetrics() *WatcherMetrics {
	watcherMetrics := &WatcherMetrics{
		certNotRenewGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: CertNotRenewMetrics,
			Help: "Indicates the Certificate CR of related Kyma is not renewed yet",
		}, []string{kymaNameLabel}),
	}
	ctrlmetrics.Registry.MustRegister(watcherMetrics.certNotRenewGauge)
	watchermetrics.Init(ctrlmetrics.Registry)
	return watcherMetrics
}

func (w *WatcherMetrics) CleanupMetrics(kymaName string) {
	w.certNotRenewGauge.DeletePartialMatch(prometheus.Labels{
		kymaNameLabel: kymaName,
	})
}

func (w *WatcherMetrics) SetCertNotRenew(kymaName string) {
	w.certNotRenewGauge.With(prometheus.Labels{
		kymaNameLabel: kymaName,
	}).Set(1)
}
