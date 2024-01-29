package metrics

import (
	"reflect"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

func Test_fetchLifecycleManagerLogs(t *testing.T) {
	var gaugeValue float64
	gaugeValue = 1
	tests := []struct {
		name                       string
		want                       []*io_prometheus_client.Metric
		wantErr                    bool
		hasLifecycleManagerMetrics bool
	}{
		{
			name: "metrics with MetricKymaState",
			want: []*io_prometheus_client.Metric{
				{
					Label: []*io_prometheus_client.LabelPair{
						{
							Name:  proto.String("label_1"),
							Value: proto.String("value_1"),
						},
					},
					Gauge: &io_prometheus_client.Gauge{
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
