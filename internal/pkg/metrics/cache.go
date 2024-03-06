package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	MetricNameCacheSizeDesc      = "lifecycle_mgr_cache_size_descriptor"
	MetricNameCacheSizeCrd       = "lifecycle_mgr_cache_size_crd"
	MetricNameCacheSizeFileMutex = "lifecycle_mgr_cache_size_filemutex"
	MetricNameCacheSizeClient    = "lifecycle_mgr_cache_size_client"
)

type CacheSizeMetrics struct {
	descriptorTotalGauge prometheus.Gauge
	crdTotalGauge        prometheus.Gauge
	filemutexTotalGauge  prometheus.Gauge
	clientTotalGauge     prometheus.Gauge
}

func NewCacheSizeMetrics() *CacheSizeMetrics {
	cacheMetrics := &CacheSizeMetrics{
		descriptorTotalGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: MetricNameCacheSizeDesc,
			Help: "Shows current number of entries in the descriptor cache",
		}),
		crdTotalGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: MetricNameCacheSizeCrd,
			Help: "Shows current number of entries in the crd cache",
		}),
		filemutexTotalGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: MetricNameCacheSizeFileMutex,
			Help: "Shows current number of entries in the filemutex cache",
		}),
		clientTotalGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: MetricNameCacheSizeClient,
			Help: "Shows current number of entries in the client cache",
		}),
	}

	ctrlmetrics.Registry.MustRegister(cacheMetrics.descriptorTotalGauge)
	ctrlmetrics.Registry.MustRegister(cacheMetrics.crdTotalGauge)
	ctrlmetrics.Registry.MustRegister(cacheMetrics.filemutexTotalGauge)
	ctrlmetrics.Registry.MustRegister(cacheMetrics.clientTotalGauge)

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

func (m *CacheSizeMetrics) UpdateClientTotal(size int) {
	m.clientTotalGauge.Set(float64(size))
}
