package shared

import (
	apimachinerymeta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LastOperation defines the last operation from the control-loop.
// +k8s:deepcopy-gen=true
type LastOperation struct {
	Operation      string                `json:"operation"`
	LastUpdateTime apimachinerymeta.Time `json:"lastUpdateTime,omitempty"`
}
