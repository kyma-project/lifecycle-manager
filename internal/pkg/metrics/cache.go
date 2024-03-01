package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	MetricNameCacheSizeDescTotal      = "lifecycle_mgr_cache_descriptor_total"
	MetricNameCacheSizeCrdTotal       = "lifecycle_mgr_cache_crd_total"
	MetricNameCacheSizeFileMutexTotal = "lifecycle_mgr_cache_filemutex_total"
)

type CacheSizeMetrics struct {
	descriptorTotalGauge prometheus.Gauge
	crdTotalGauge        prometheus.Gauge
	filemutexTotalGauge  prometheus.Gauge
}

func NewCacheSizeMetrics() *CacheSizeMetrics {
	cacheMetrics := &CacheSizeMetrics{
		descriptorTotalGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: MetricNameCacheSizeDescTotal,
			Help: "Shows current number of entries in the descriptor cache",
		}),
		crdTotalGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: MetricNameCacheSizeCrdTotal,
			Help: "Shows current number of entries in the crd cache",
		}),
		filemutexTotalGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: MetricNameCacheSizeFileMutexTotal,
			Help: "Shows current number of entries in the filemutex cache",
		}),
	}

	ctrlmetrics.Registry.MustRegister(cacheMetrics.descriptorTotalGauge)
	ctrlmetrics.Registry.MustRegister(cacheMetrics.crdTotalGauge)
	ctrlmetrics.Registry.MustRegister(cacheMetrics.filemutexTotalGauge)

	return cacheMetrics
}

func (m *CacheSizeMetrics) UpdateDescriptorTotal(size int) {
	m.descriptorTotalGauge.Set(float64(size))
}

func (m *CacheSizeMetrics) UpdateCrdTotal(size int) {
	m.crdTotalGauge.Set(float64(size))
}

func (m *CacheSizeMetrics) UpdateFilemutexTotal(size int) {
	m.filemutexTotalGauge.Set(float64(size))
}
