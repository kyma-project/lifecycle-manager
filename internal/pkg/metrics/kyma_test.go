package metrics_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	prometheusclient "github.com/prometheus/client_model/go"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	machineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
)

func TestKymaMetrics_CleanupNonExistingKymaCrsMetrics(t *testing.T) {
	t.Parallel()
	kymaName := "kyma-sample"
	sampleGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: metrics.MetricKymaState,
		Help: "Indicates the Status.state for a given Kyma object",
	}, []string{"kyma_name"})
	deployedKymas := &v1beta2.KymaList{
		Items: []v1beta2.Kyma{
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name: kymaName,
				},
			},
		},
	}
	sc := machineryruntime.NewScheme()
	machineryutilruntime.Must(v1beta2.AddToScheme(sc))

	fakeClientBuilder := fake.NewClientBuilder().WithScheme(sc).WithRuntimeObjects(deployedKymas).Build()
	ctrlmetrics.Registry.Unregister(sampleGauge)
	ctrlmetrics.Registry.MustRegister(sampleGauge)
	sampleGauge.With(prometheus.Labels{
		metrics.KymaNameLabel: kymaName,
	}).Set(1)
	sampleGauge.With(prometheus.Labels{
		metrics.KymaNameLabel: "non-existing",
	}).Set(1)

	kymaNameLabel := metrics.KymaNameLabel

	gaugeValue := 1.0
	wantResultingMetrics := []*prometheusclient.Metric{
		{
			Label: []*prometheusclient.LabelPair{
				{
					Name:  &kymaNameLabel,
					Value: &kymaName,
				},
			},
			Gauge: &prometheusclient.Gauge{
				Value: &gaugeValue,
			},
		},
	}

	resultingMetrics, _ := ctrlmetrics.Registry.Gather()
	for _, metric := range resultingMetrics {
		if metric.GetName() == metrics.MetricKymaState {
			if len(metric.GetMetric()) < 2 {
				t.Errorf("failed to set the non-existing kyma")
			}
		}
	}

	k := &metrics.KymaMetrics{
		KymaStateGauge: sampleGauge,
	}
	if err := k.CleanupNonExistingKymaCrsMetrics(context.TODO(), fakeClientBuilder); err != nil {
		t.Errorf("CleanupNonExistingKymaCrsMetrics() error = %v", err)
	}

	foundKymaStateMetric := false
	resultingMetrics, _ = ctrlmetrics.Registry.Gather()
	for _, metric := range resultingMetrics {
		if metric.GetName() == metrics.MetricKymaState {
			foundKymaStateMetric = true
			if !reflect.DeepEqual(metric.GetMetric(), wantResultingMetrics) {
				t.Errorf("resultMetrics: got = %v, want %v", metric.GetMetric(),
					wantResultingMetrics)
			}
		}
	}

	if !foundKymaStateMetric {
		t.Errorf("expected to have lifecycle_mgr_kyma_state but no kyma metric was found")
	}
}
