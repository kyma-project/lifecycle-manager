package shared

import (
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LastOperation defines the last operation from the control-loop.
// +kubebuilder:object:generate=true
type LastOperation struct {
	Operation      string         `json:"operation"`
	LastUpdateTime apimetav1.Time `json:"lastUpdateTime,omitempty"`
}
