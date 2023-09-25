package metrics

import (
	"errors"
	"fmt"
	"time"

	ctrlMetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/controller/common/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricPurgeTime                     = "lifecycle_mgr_purgectrl_time"
	metricPurgeRequests                 = "lifecycle_mgr_purgectrl_requests"
	kymaNameLabel                       = "kyma_name"
	shootIDLabel                        = "shoot"
	instanceIDLabel                     = "instance_id"
	errorReasonLabel                    = "err_reason"
	ErrPurgeFinalizerRemoval PurgeError = "PurgeFinalizerRemovalError"
	ErrCleanup               PurgeError = "CleanupError"
)

type PurgeError string

var (
	purgeTimeGauge = prometheus.NewGauge(prometheus.GaugeOpts{ //nolint:gochecknoglobals
		Name: metricPurgeTime,
		Help: "Indicates average purge duration",
	})
	purgeRequestsCounter = prometheus.NewCounter(prometheus.CounterOpts{ //nolint:gochecknoglobals
		Name: metricPurgeRequests,
		Help: "Indicates total purge count ",
	})
	purgeErrorGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{ //nolint:gochecknoglobals
		Name: metricPurgeRequests,
		Help: "Indicates purge errors",
	}, []string{kymaNameLabel, shootIDLabel, instanceIDLabel})
)

func Initialize() {
	ctrlMetrics.Registry.MustRegister(purgeTimeGauge)
	ctrlMetrics.Registry.MustRegister(purgeRequestsCounter)
	ctrlMetrics.Registry.MustRegister(purgeErrorGauge)
}

var errMetric = errors.New("failed to update metrics")

func UpdatePurgeCount() {
	purgeRequestsCounter.Inc()
}

func UpdatePurgeTime(duration time.Duration) {
	purgeTimeGauge.Set(duration.Seconds())
}

func UpdatePurgeError(kyma *v1beta2.Kyma, purgeError PurgeError) error {
	shootID, err := metrics.ExtractShootID(kyma)
	if err != nil {
		return fmt.Errorf("%w: %w", errMetric, err)
	}
	instanceID, err := metrics.ExtractInstanceID(kyma)
	if err != nil {
		return fmt.Errorf("%w: %w", errMetric, err)
	}
	purgeErrorGauge.With(prometheus.Labels{
		kymaNameLabel:    kyma.Name,
		shootIDLabel:     shootID,
		instanceIDLabel:  instanceID,
		errorReasonLabel: string(purgeError),
	}).Set(1)

	return nil
}
