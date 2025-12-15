package metrics_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	prometheusclient "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	machineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

var labelNames = []string{
	metrics.KymaNameLabel,
	random.Name(),
	random.Name(),
	random.Name(),
}

func TestKymaMetrics_CleanupNonExistingKymaCrsMetrics(t *testing.T) {
	kymaName := "kyma-sample"
	sampleGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: metrics.MetricKymaState,
	}, labelNames)
	deployedKymas := &v1beta2.KymaList{
		Items: []v1beta2.Kyma{
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name: kymaName,
				},
			},
		},
	}
	scheme := machineryruntime.NewScheme()
	machineryutilruntime.Must(v1beta2.AddToScheme(scheme))

	labelValues := []string{
		kymaName,
		random.Name(),
		random.Name(),
		random.Name(),
	}

	fakeClientBuilder := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(deployedKymas).Build()
	ctrlmetrics.Registry.Unregister(sampleGauge)
	ctrlmetrics.Registry.MustRegister(sampleGauge)
	// this metric should be kept
	sampleGauge.With(prometheus.Labels{
		labelNames[0]: labelValues[0],
		labelNames[1]: labelValues[1],
		labelNames[2]: labelValues[2],
		labelNames[3]: labelValues[3],
	}).Set(1)
	// these metrics should be dropped
	populateRandomMetrics(sampleGauge, labelNames, 10)

	gaugeValue := 1.0
	wantResultingMetrics := []*prometheusclient.Metric{
		{
			Label: []*prometheusclient.LabelPair{
				{
					Name:  &labelNames[0],
					Value: &labelValues[0],
				},
				{
					Name:  &labelNames[1],
					Value: &labelValues[1],
				},
				{
					Name:  &labelNames[2],
					Value: &labelValues[2],
				},
				{
					Name:  &labelNames[3],
					Value: &labelValues[3],
				},
			},
			Gauge: &prometheusclient.Gauge{
				Value: &gaugeValue,
			},
		},
	}

	receivedMetrics, _ := ctrlmetrics.Registry.Gather()
	for _, metric := range receivedMetrics {
		if metric.GetName() == metrics.MetricKymaState {
			if len(metric.GetMetric()) < 2 {
				t.Errorf("failed to set the non-existing kyma")
			}
		}
	}

	k := &metrics.KymaMetrics{
		KymaStateGauge: sampleGauge,
	}
	if err := k.CleanupNonExistingKymaCrsMetrics(t.Context(), fakeClientBuilder); err != nil {
		t.Errorf("CleanupNonExistingKymaCrsMetrics() error = %v", err)
	}

	foundKymaStateMetric := false
	receivedMetrics, _ = ctrlmetrics.Registry.Gather()
	for _, receivedMetric := range receivedMetrics {
		if receivedMetric.GetName() == metrics.MetricKymaState {
			foundKymaStateMetric = true
			for _, rm := range receivedMetric.GetMetric() {
				for _, wm := range wantResultingMetrics {
					assert.ElementsMatch(t, wm.GetLabel(), rm.GetLabel())
					assert.Equal(t, wm.GetGauge(), rm.GetGauge())
				}
			}
		}
	}

	if !foundKymaStateMetric {
		t.Errorf("expected to have lifecycle_mgr_kyma_state but no kyma metric was found")
	}
}

func Test_HasMetrics_True_ForKymaWithMetrics(t *testing.T) {
	metricNames := []string{
		metrics.MetricKymaState,
		metrics.MetricModuleState,
	}

	for _, metricName := range metricNames {
		t.Run(metricName, func(t *testing.T) {
			kymaName := random.Name()

			gauge := createTestGauge(metricName, labelNames)
			populateRandomMetrics(gauge, labelNames, 10)

			// add one metric with expected value in kyma name label
			gauge.With(prometheus.Labels{
				labelNames[0]: kymaName,
				labelNames[1]: random.Name(),
				labelNames[2]: random.Name(),
				labelNames[3]: random.Name(),
			}).Set(1)

			kymaMetrics := metrics.KymaMetrics{}

			hasMetrics, err := kymaMetrics.HasMetrics(kymaName)

			require.NoError(t, err)
			assert.True(t, hasMetrics)
		})
	}
}

func Test_HasMetrics_False_ForKymaWithNoMetrics(t *testing.T) {
	metricNames := []string{
		metrics.MetricKymaState,
		metrics.MetricModuleState,
	}

	for _, metricName := range metricNames {
		t.Run(metricName, func(t *testing.T) {
			kymaName := random.Name()

			// no metric with expected value kyma name label
			gauge := createTestGauge(metricName, labelNames)
			populateRandomMetrics(gauge, labelNames, 10)

			kymaMetrics := metrics.KymaMetrics{}

			hasMetrics, err := kymaMetrics.HasMetrics(kymaName)

			require.NoError(t, err)
			assert.False(t, hasMetrics)
		})
	}
}

func createTestGauge(metricName string, labelNames []string) *prometheus.GaugeVec {
	testGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricName,
	}, labelNames)

	ctrlmetrics.Registry.Unregister(testGauge)
	ctrlmetrics.Registry.MustRegister(testGauge)

	return testGauge
}

func populateRandomMetrics(gauge *prometheus.GaugeVec, labelNames []string, count int) {
	for range count {
		labels := make(prometheus.Labels)
		for _, labelName := range labelNames {
			labels[labelName] = random.Name()
		}
		gauge.With(labels).Set(1)
	}
}
