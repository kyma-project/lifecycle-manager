package metrics

import (
	"errors"
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	ctrlMetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

const (
	metricKymaState   = "lifecycle_mgr_kyma_state"
	metricModuleState = "lifecycle_mgr_module_state"
	kymaNameLabel     = "kyma_name"
	stateLabel        = "state"
	shootIDLabel      = "shoot"
	instanceIDLabel   = "instance_id"
	moduleNameLabel   = "module_name"
)

var (
	kymaStateGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{ //nolint:gochecknoglobals
		Name: metricKymaState,
		Help: "Indicates the Status.state for a given Kyma object",
	}, []string{kymaNameLabel, stateLabel, shootIDLabel, instanceIDLabel})
	moduleStateGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{ //nolint:gochecknoglobals
		Name: metricModuleState,
		Help: "Indicates the Status.state for modules of Kyma",
	}, []string{moduleNameLabel, kymaNameLabel, stateLabel, shootIDLabel, instanceIDLabel})
)

func Initialize() {
	requestLatency := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: "rest_client",
			Name:      "request_latency_seconds",
			Help:      "Request latency in seconds. Broken down by verb and URL.",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"verb", "url"},
	)
	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rest_client_request_duration_seconds",
			Help:    "Request latency in seconds. Broken down by verb, and host.",
			Buckets: []float64{0.005, 0.025, 0.1, 0.25, 0.5, 1.0, 2.0, 4.0, 8.0, 15.0, 30.0, 60.0},
		},
		[]string{"verb", "host"},
	)
	requestSize := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rest_client_request_size_bytes",
			Help:    "Request size in bytes. Broken down by verb and host.",
			Buckets: []float64{64, 256, 512, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216},
		},
		[]string{"verb", "host"},
	)
	responseSize := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rest_client_response_size_bytes",
			Help:    "Response size in bytes. Broken down by verb and host.",
			Buckets: []float64{64, 256, 512, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216},
		},
		[]string{"verb", "host"},
	)

	ctrlMetrics.Registry.Unregister(requestLatency)
	ctrlMetrics.Registry.Unregister(requestDuration)
	ctrlMetrics.Registry.Unregister(requestSize)
	ctrlMetrics.Registry.Unregister(responseSize)

	ctrlMetrics.Registry.MustRegister(kymaStateGauge)
	ctrlMetrics.Registry.MustRegister(moduleStateGauge)
}

var errMetric = errors.New("failed to update metrics")

// UpdateAll sets both metrics 'lifecycle_mgr_kyma_state' and 'lifecycle_mgr_module_state' to new states.
func UpdateAll(kyma *v1beta2.Kyma) error {
	shootID, err := extractShootID(kyma)
	if err != nil {
		return errors.Join(errMetric, err)
	}
	instanceID, err := extractInstanceID(kyma)
	if err != nil {
		return errors.Join(errMetric, err)
	}

	setKymaStateGauge(kyma.Status.State, kyma.Name, shootID, instanceID)
	for _, moduleStatus := range kyma.Status.Modules {
		setModuleStateGauge(moduleStatus.State, moduleStatus.Name, kyma.Name, shootID, instanceID)
	}
	return nil
}

// RemoveKymaStateMetrics deletes all 'lifecycle_mgr_kyma_state' metrics for the matching Kyma.
func RemoveKymaStateMetrics(kyma *v1beta2.Kyma) error {
	shootID, err := extractShootID(kyma)
	if err != nil {
		return err
	}
	instanceID, err := extractInstanceID(kyma)
	if err != nil {
		return err
	}

	kymaStateGauge.DeletePartialMatch(prometheus.Labels{
		kymaNameLabel:   kyma.Name,
		shootIDLabel:    shootID,
		instanceIDLabel: instanceID,
	})
	return nil
}

// RemoveModuleStateMetrics deletes all 'lifecycle_mgr_module_state' metrics for the matching module.
func RemoveModuleStateMetrics(kyma *v1beta2.Kyma, moduleName string) error {
	shootID, err := extractShootID(kyma)
	if err != nil {
		return err
	}
	instanceID, err := extractInstanceID(kyma)
	if err != nil {
		return err
	}

	moduleStateGauge.DeletePartialMatch(prometheus.Labels{
		moduleNameLabel: moduleName,
		kymaNameLabel:   kyma.Name,
		shootIDLabel:    shootID,
		instanceIDLabel: instanceID,
	})
	return nil
}

func setKymaStateGauge(newState v1beta2.State, kymaName, shootID, instanceID string) {
	states := v1beta2.AllKymaStates()
	for _, state := range states {
		newValue := calcStateValue(state, newState)
		kymaStateGauge.With(prometheus.Labels{
			kymaNameLabel:   kymaName,
			shootIDLabel:    shootID,
			instanceIDLabel: instanceID,
			stateLabel:      string(state),
		}).Set(newValue)
	}
}

func setModuleStateGauge(newState v1beta2.State, moduleName, kymaName, shootID, instanceID string) {
	states := v1beta2.AllKymaStates()
	for _, state := range states {
		newValue := calcStateValue(state, newState)
		moduleStateGauge.With(prometheus.Labels{
			moduleNameLabel: moduleName,
			kymaNameLabel:   kymaName,
			shootIDLabel:    shootID,
			instanceIDLabel: instanceID,
			stateLabel:      string(state),
		}).Set(newValue)
	}
}

var (
	errMissingShootAnnotation = fmt.Errorf("expected annotation '%s' not found", v1beta2.SKRDomainAnnotation)
	errShootAnnotationNoValue = fmt.Errorf("annotation '%s' has empty value", v1beta2.SKRDomainAnnotation)
)

func extractShootID(kyma *v1beta2.Kyma) (string, error) {
	shoot := ""
	shootFQDN, keyExists := kyma.Annotations[v1beta2.SKRDomainAnnotation]
	if keyExists {
		parts := strings.Split(shootFQDN, ".")
		minFqdnParts := 2
		if len(parts) > minFqdnParts {
			shoot = parts[0] // hostname
		}
	}
	if !keyExists {
		return "", errMissingShootAnnotation
	}
	if shoot == "" {
		return shoot, errShootAnnotationNoValue
	}
	return shoot, nil
}

var (
	errMissingInstanceLabel = fmt.Errorf("expected label '%s' not found", v1beta2.InstanceIDLabel)
	errInstanceLabelNoValue = fmt.Errorf("label '%s' has empty value", v1beta2.InstanceIDLabel)
)

func extractInstanceID(kyma *v1beta2.Kyma) (string, error) {
	instanceID, keyExists := kyma.Labels[v1beta2.InstanceIDLabel]
	if !keyExists {
		return "", errMissingInstanceLabel
	}
	if instanceID == "" {
		return instanceID, errInstanceLabelNoValue
	}
	return instanceID, nil
}

func calcStateValue(state, newState v1beta2.State) float64 {
	if state == newState {
		return 1
	}
	return 0
}
