package metrics

import (
	"errors"
	"fmt"
	"strings"

	listenerMetrics "github.com/kyma-project/runtime-watcher/listener/pkg/metrics"
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
	ctrlMetrics.Registry.MustRegister(kymaStateGauge)
	ctrlMetrics.Registry.MustRegister(moduleStateGauge)
	listenerMetrics.InitMetrics(ctrlMetrics.Registry)
}

var errMetric = errors.New("failed to update metrics")

// UpdateAll sets both metrics 'lifecycle_mgr_kyma_state' and 'lifecycle_mgr_module_state' to new states.
func UpdateAll(kyma *v1beta2.Kyma) error {
	shootID, err := extractShootID(kyma)
	if err != nil {
		return fmt.Errorf("%w: %w", errMetric, err)
	}
	instanceID, err := extractInstanceID(kyma)
	if err != nil {
		return fmt.Errorf("%w: %w", errMetric, err)
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
