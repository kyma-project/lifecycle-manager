package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	MetricNameCacheSizeDescTotal      = "lifecycle_mgr_cache_descriptor_total"
	MetricNameCacheSizeDescBytes      = "lifecycle_mgr_cache_descriptor_bytes"
	MetricNameCacheSizeCrdTotal       = "lifecycle_mgr_cache_crd_total"
	MetricNameCacheSizeCrdBytes       = "lifecycle_mgr_cache_crd_bytes"
	MetricNameCacheSizeFileMutexTotal = "lifecycle_mgr_cache_filemutex_total"
	MetricNameCacheSizeFileMutexBytes = "lifecycle_mgr_cache_filemutex_bytes"
)

type CacheSizeMetrics struct {
	descriptorTotalGauge prometheus.Gauge
	descriptorBytesGauge prometheus.Gauge
	crdTotalGauge        prometheus.Gauge
	crdBytesGauge        prometheus.Gauge
	filemutexTotalGauge  prometheus.Gauge
	filemutexBytesGauge  prometheus.Gauge
}

func NewCacheSizeMetrics() *CacheSizeMetrics {
	cacheMetrics := &CacheSizeMetrics{
		descriptorTotalGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: MetricNameCacheSizeDescTotal,
			Help: "Shows current number of entries in the descriptor cache",
		}),
		descriptorBytesGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: MetricNameCacheSizeDescBytes,
			Help: "Shows current descriptor cache size in bytes",
		}),
		crdTotalGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: MetricNameCacheSizeCrdTotal,
			Help: "Shows current number of entries in the crd cache",
		}),
		crdBytesGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: MetricNameCacheSizeCrdBytes,
			Help: "Shows current crd cache size in bytes",
		}),
		filemutexTotalGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: MetricNameCacheSizeFileMutexTotal,
			Help: "Shows current number of entries in the filemutex cache",
		}),
		filemutexBytesGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: MetricNameCacheSizeFileMutexBytes,
			Help: "Shows current filemutex cache size in bytes",
		}),
	}

	ctrlmetrics.Registry.MustRegister(cacheMetrics.descriptorTotalGauge)
	ctrlmetrics.Registry.MustRegister(cacheMetrics.descriptorBytesGauge)
	ctrlmetrics.Registry.MustRegister(cacheMetrics.crdTotalGauge)
	ctrlmetrics.Registry.MustRegister(cacheMetrics.crdBytesGauge)
	ctrlmetrics.Registry.MustRegister(cacheMetrics.filemutexTotalGauge)
	ctrlmetrics.Registry.MustRegister(cacheMetrics.filemutexBytesGauge)

	return cacheMetrics
}

func (m *CacheSizeMetrics) UpdateDescriptorTotal(size int) {
	m.descriptorTotalGauge.Set(float64(size))
}

func (m *CacheSizeMetrics) UpdateDescriptorBytes(size int) {
	m.descriptorBytesGauge.Set(float64(size))
}

func (m *CacheSizeMetrics) UpdateCrdTotal(size int) {
	m.crdTotalGauge.Set(float64(size))
}

func (m *CacheSizeMetrics) UpdateCrdBytes(size int) {
	m.crdBytesGauge.Set(float64(size))
}

func (m *CacheSizeMetrics) UpdateFilemutexTotal(size int) {
	m.filemutexTotalGauge.Set(float64(size))
}

func (m *CacheSizeMetrics) UpdateFilemutexBytes(size int) {
	m.filemutexBytesGauge.Set(float64(size))
}
