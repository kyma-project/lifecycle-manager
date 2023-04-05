package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlMetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	kymaTypes "github.com/kyma-project/lifecycle-manager/api/v1beta1"
)

const (
	kymaNameLabel   = "kyma_name"
	stateLabel      = "state"
	shootLabel      = "shoot"
	instanceIDLabel = "instance_id"
)

// KymaStatus is a prometheus metric which holds
// a count for Status.state value for every reconciled Kyma.
// The value of zero means the status is not set, the value of 1 means the status is set.
// The "state" label values must be one of the defined Status.state values for Kyma CRs.
var kymaStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{ //nolint:gochecknoglobals
	Name: "lifecycle_manager_kyma_status",
	Help: "Indicates the Status.state for a given Kyma object",
}, []string{kymaNameLabel, stateLabel, shootLabel, instanceIDLabel})

func Initialize() {
	ctrlMetrics.Registry.MustRegister(kymaStatus)
}

// RecordKymaStatus adjusts the metric that tracks the current "Status.state" of the Kyma object.
func RecordKymaStatus(kymaName string, newState kymaTypes.State, shoot, instanceID string) {
	for _, definedState := range kymaTypes.AllKymaStates() {
		mtr := kymaStatus.With(
			prometheus.Labels{
				kymaNameLabel:   kymaName,
				stateLabel:      string(newState),
				shootLabel:      shoot,
				instanceIDLabel: instanceID,
			})
		if newState == definedState {
			mtr.Set(1)
		} else {
			mtr.Set(0)
		}
	}
}
