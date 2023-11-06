package shared

import (
	"time"

	apimachinerymeta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Status defines the observed state of CustomObject.
// +k8s:deepcopy-gen=true
type Status struct {
	// State signifies current state of CustomObject.
	// Value can be one of ("Ready", "Processing", "Error", "Deleting", "Warning").
	// +kubebuilder:validation:Required
	State State `json:"state,omitempty"`

	// Conditions contain a set of conditionals to determine the State of Status.
	// If all Conditions are met, the State is expected to be in StateReady.
	// +listType=map
	// +listMapKey=type
	Conditions []apimachinerymeta.Condition `json:"conditions,omitempty" patchMergeKey:"type" patchStrategy:"merge"`

	// Synced determine a list of Resources that are currently actively synced.
	// All resources that are synced are considered for orphan removal on configuration changes,
	// and it is used to determine effective differences from one state to the next.
	// +listType=atomic
	Synced        []Resource `json:"synced,omitempty"`
	LastOperation `json:"lastOperation,omitempty"`
}

func (s Status) WithState(state State) Status {
	s.State = state
	return s
}

func (s Status) WithErr(err error) Status {
	s.LastOperation = LastOperation{Operation: err.Error(), LastUpdateTime: apimachinerymeta.NewTime(time.Now())}
	return s
}

func (s Status) WithOperation(operation string) Status {
	s.LastOperation = LastOperation{Operation: operation, LastUpdateTime: apimachinerymeta.NewTime(time.Now())}
	return s
}
