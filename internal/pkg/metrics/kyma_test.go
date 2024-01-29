package metrics

import (
	"context"
	"reflect"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/prometheus/client_golang/prometheus"
	prometheusclient "github.com/prometheus/client_model/go"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	machineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func Test_fetchLifecycleManagerLogs(t *testing.T) {
	var gaugeValue float64
	gaugeValue = 1
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
							Name:  proto.String("label_1"),
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
		t.Run(tt.name, func(t *testing.T) {
			sampleGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: MetricKymaState,
				Help: "Indicates the Status.state for a given Kyma object",
			}, []string{"label_1"})
			if tt.hasLifecycleManagerMetrics {
				ctrlmetrics.Registry.MustRegister(sampleGauge)
				sampleGauge.With(prometheus.Labels{
					"label_1": "value_1",
				}).Set(1)
			} else {
				ctrlmetrics.Registry.Unregister(sampleGauge)
			}
			got, err := fetchLifecycleManagerMetrics()
			if (err != nil) != tt.wantErr {
				t.Errorf("fetchLifecycleManagerLogs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("fetchLifecycleManagerLogs() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKymaMetrics_CleanupNonExistingKymaCrsMetrics(t *testing.T) {
	deployedKymas := &v1beta2.KymaList{
		Items: []v1beta2.Kyma{
			{
				ObjectMeta: v1.ObjectMeta{
					Name: "kyma-sample",
				},
			},
		},
	}
	sc := runtime.NewScheme()
	machineryutilruntime.Must(v1beta2.AddToScheme(sc))

	fakeClientBuilder := fake.NewClientBuilder().WithScheme(sc).WithRuntimeObjects(deployedKymas).Build()
	sampleGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: MetricKymaState,
		Help: "Indicates the Status.state for a given Kyma object",
	}, []string{KymaNameLabel})
	ctrlmetrics.Registry.MustRegister(sampleGauge)
	sampleGauge.With(prometheus.Labels{
		KymaNameLabel: "kyma-sample",
	}).Set(1)

	var gaugeValue float64
	gaugeValue = 1

	type fields struct {
		kymaStateGauge *prometheus.GaugeVec
		SharedMetrics  *SharedMetrics
	}
	type args struct {
		ctx       context.Context
		kcpClient client.Client
	}
	tests := []struct {
		name                 string
		args                 args
		fields               fields
		wantErr              bool
		hasNonExistingKyma   bool
		wantResultingMetrics []*prometheusclient.Metric
	}{

		{
			name:    "Metrics without non-existing kymas",
			wantErr: false,
			args: args{
				ctx:       context.TODO(),
				kcpClient: fakeClientBuilder,
			},
			fields:             fields{kymaStateGauge: sampleGauge},
			hasNonExistingKyma: false,
			wantResultingMetrics: []*prometheusclient.Metric{
				{
					Label: []*prometheusclient.LabelPair{
						{
							Name:  proto.String(KymaNameLabel),
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
			name:    "Metrics with non-existing kymas",
			wantErr: false,
			args: args{
				ctx:       context.TODO(),
				kcpClient: fakeClientBuilder,
			},
			fields:             fields{kymaStateGauge: sampleGauge},
			hasNonExistingKyma: true,
			wantResultingMetrics: []*prometheusclient.Metric{
				{
					Label: []*prometheusclient.LabelPair{
						{
							Name:  proto.String(KymaNameLabel),
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
		t.Run(tt.name, func(t *testing.T) {
			if tt.hasNonExistingKyma {
				sampleGauge.With(prometheus.Labels{
					KymaNameLabel: "non-existing",
				}).Set(1)

				resultingMetrics, _ := ctrlmetrics.Registry.Gather()
				for _, metric := range resultingMetrics {
					if metric.GetName() == MetricKymaState {
						if len(metric.Metric) != 2 {
							t.Errorf("failed to set the non-existing kyma")
						}
					}
				}
			}

			k := &KymaMetrics{
				kymaStateGauge: tt.fields.kymaStateGauge,
			}
			if err := k.CleanupNonExistingKymaCrsMetrics(tt.args.ctx, tt.args.kcpClient); (err != nil) != tt.wantErr {
				t.Errorf("CleanupNonExistingKymaCrsMetrics() error = %v, wantErr %v", err, tt.wantErr)
			}

			resultingMetrics, _ := ctrlmetrics.Registry.Gather()
			for _, metric := range resultingMetrics {
				if metric.GetName() == MetricKymaState {
					if !reflect.DeepEqual(metric.Metric, tt.wantResultingMetrics) {
						t.Errorf("resultMetrics: got = %v, want %v", metric.Metric,
							tt.wantResultingMetrics)
					}
				}
			}
		})
	}
}
