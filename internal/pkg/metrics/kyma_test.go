package metrics_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/prometheus/client_golang/prometheus"
	prometheusclient "github.com/prometheus/client_model/go"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	machineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
)

var sampleGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: metrics.MetricKymaState,
	Help: "Indicates the Status.state for a given Kyma object",
}, []string{"kyma_name"})

func Test_fetchLifecycleManagerLogs(t *testing.T) {
	t.Parallel()
	gaugeValue := 1.0
	tests := []struct {
		name                       string
		want                       []*prometheusclient.Metric
		wantErr                    bool
		hasLifecycleManagerMetrics bool
	}{
		{
			name: "metrics with MetricKymaState",
			want: []*prometheusclient.Metric{
				{
					Label: []*prometheusclient.LabelPair{
						{
							Name:  proto.String("kyma_name"),
							Value: proto.String("value_1"),
						},
					},
					Gauge: &prometheusclient.Gauge{
						Value: &gaugeValue,
					},
				},
			},
			wantErr:                    false,
			hasLifecycleManagerMetrics: true,
		},
		{
			name:                       "metrics with no MetricKymaState",
			want:                       nil,
			wantErr:                    false,
			hasLifecycleManagerMetrics: false,
		},
	}
	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if test.hasLifecycleManagerMetrics {
				ctrlmetrics.Registry.Unregister(sampleGauge)
				ctrlmetrics.Registry.MustRegister(sampleGauge)
				sampleGauge.With(prometheus.Labels{
					"kyma_name": "value_1",
				}).Set(1)
			} else {
				ctrlmetrics.Registry.Unregister(sampleGauge)
			}
			got, err := metrics.FetchLifecycleManagerMetrics()
			if (err != nil) != test.wantErr {
				t.Errorf("fetchLifecycleManagerLogs() error = %v, wantErr %v", err, test.wantErr)
				return
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("fetchLifecycleManagerLogs() got = %v, want %v", got, test.want)
			}
		})
	}
}

func TestKymaMetrics_CleanupNonExistingKymaCrsMetrics(t *testing.T) {
	t.Parallel()
	deployedKymas := &v1beta2.KymaList{
		Items: []v1beta2.Kyma{
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name: "kyma-sample",
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
		metrics.KymaNameLabel: "kyma-sample",
	}).Set(1)

	gaugeValue := 1.0

	tests := []struct {
		name                 string
		kcpClient            client.Client
		kymaStateGauge       *prometheus.GaugeVec
		wantErr              bool
		hasNonExistingKyma   bool
		wantResultingMetrics []*prometheusclient.Metric
	}{
		{
			name:               "Metrics without non-existing kymas",
			wantErr:            false,
			kcpClient:          fakeClientBuilder,
			kymaStateGauge:     sampleGauge,
			hasNonExistingKyma: false,
			wantResultingMetrics: []*prometheusclient.Metric{
				{
					Label: []*prometheusclient.LabelPair{
						{
							Name:  proto.String(metrics.KymaNameLabel),
							Value: proto.String("kyma-sample"),
						},
					},
					Gauge: &prometheusclient.Gauge{
						Value: &gaugeValue,
					},
				},
			},
		},
		{
			name:               "Metrics with non-existing kymas",
			wantErr:            false,
			kcpClient:          fakeClientBuilder,
			kymaStateGauge:     sampleGauge,
			hasNonExistingKyma: true,
			wantResultingMetrics: []*prometheusclient.Metric{
				{
					Label: []*prometheusclient.LabelPair{
						{
							Name:  proto.String(metrics.KymaNameLabel),
							Value: proto.String("kyma-sample"),
						},
					},
					Gauge: &prometheusclient.Gauge{
						Value: &gaugeValue,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if test.hasNonExistingKyma {
				if err := registerNonExistingKymaMetrics(); err != nil {
					t.Errorf("failed to set the non-existing kyma")
				}
			}

			k := &metrics.KymaMetrics{
				KymaStateGauge: test.kymaStateGauge,
			}
			if err := k.CleanupNonExistingKymaCrsMetrics(context.TODO(), test.kcpClient); (err != nil) != test.wantErr {
				t.Errorf("CleanupNonExistingKymaCrsMetrics() error = %v, wantErr %v", err, test.wantErr)
			}

			resultingMetrics, _ := ctrlmetrics.Registry.Gather()
			for _, metric := range resultingMetrics {
				if metric.GetName() == metrics.MetricKymaState {
					if !reflect.DeepEqual(metric.GetMetric(), test.wantResultingMetrics) {
						t.Errorf("resultMetrics: got = %v, want %v", metric.GetMetric(),
							test.wantResultingMetrics)
					}
				}
			}
		})
	}
}

func registerNonExistingKymaMetrics() error {
	sampleGauge.With(prometheus.Labels{
		metrics.KymaNameLabel: "non-existing",
	}).Set(1)

	resultingMetrics, _ := ctrlmetrics.Registry.Gather()
	for _, metric := range resultingMetrics {
		if metric.GetName() == metrics.MetricKymaState {
			if len(metric.GetMetric()) != 2 {
				return fmt.Errorf("failed to set the non-existing Kymas")
			}
		}
	}

	return nil
}
