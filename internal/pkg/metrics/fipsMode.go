package metrics

import (
	"crypto/fips140"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	MetricFipsMode = "lifecycle_mgr_fips_mode" // Name of the FIPS mode metric
)

type FipsModeMetrics struct {
	fipsModeGauge prometheus.Gauge
}

func NewFipsMetrics() *FipsModeMetrics {
	res := &FipsModeMetrics{
		fipsModeGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: MetricFipsMode,
			Help: "current FIPS mode (0=off/1=on/2=only)",
		}),
	}
	ctrlmetrics.Registry.MustRegister(res.fipsModeGauge)
	return res
}

func (f *FipsModeMetrics) Update() {
	if f == nil || f.fipsModeGauge == nil {
		return
	}

	fipsMode := 0
	if fips140.Enabled() {
		if parseGodebugFipsMode(os.Getenv("GODEBUG")) == "only" {
			fipsMode = 2 // FIPS 140-3 only mode
		} else {
			fipsMode = 1 // FIPS 140-3 enabled
		}
	}
	f.fipsModeGauge.Set(float64(fipsMode))
}

// parseGodebugFipsMode parses the provided value to determine the FIPS mode.
// The argument to this function should be a value of the GODEBUG environment variable.
// We need to parse it manually because the provided fips140.Enabled() function doesn't distinguish
// between "on", "debug" and "only" modes.
func parseGodebugFipsMode(goDebugEnvVar string) string {
	const off = "off"

	result := off

	if len(goDebugEnvVar) == 0 {
		return result
	}
	for keyValuePair := range strings.SplitSeq(goDebugEnvVar, ",") {
		keyValuePair = strings.TrimSpace(keyValuePair)
		if justValue := strings.TrimPrefix(keyValuePair, "fips140="); len(justValue) < len(keyValuePair) {
			result = justValue
			// We're not leaving the loop here to align with the Go standard library parsing logic,
			// where "the last provided value wins"
		}
	}
	switch result {
	case off, "on", "debug", "only":
		return result
	default:
		// If the value is not one of the expected values, return "off" to have exhaustive matching.
		// Go standard library panics in this case.
		return off
	}
}
