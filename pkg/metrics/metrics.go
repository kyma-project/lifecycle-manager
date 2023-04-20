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

var (
	// KymaStatusInfo is a prometheus metric which holds
	// a count for Status.state value for every reconciled Kyma.
	// The value of zero means the status is not set, the value of 1 means the status is set.
	// The "state" label values must be one of the defined Status.state values for Kyma CRs.
	KymaStatusInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{ //nolint:gochecknoglobals
		Name: "klm_kyma_status_info",
		Help: "Indicates the Status.state for a given Kyma object",
	}, []string{kymaNameLabel, stateLabel, shootLabel, instanceIDLabel})
)

func Initialize() {
	ctrlMetrics.Registry.MustRegister(KymaStatusInfo)
}

// SetKymaStatusInfo adjusts the metric that tracks the current "Status.state" of the Kyma object.
func SetKymaStatusInfo(new kymaTypes.State, kymaName, shoot, instanceID string) {
	states := kymaTypes.AllKymaStates()
	for _, state := range states {
		mtr := KymaStatusInfo.With(
			prometheus.Labels{
				kymaNameLabel:   kymaName,
				stateLabel:      string(state),
				shootLabel:      shoot,
				instanceIDLabel: instanceID,
			})
		if state == new {
			mtr.Set(1)
		} else {
			mtr.Set(0)
		}
	}
}
