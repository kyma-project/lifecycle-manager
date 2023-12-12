/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta2

import (
	"k8s.io/apimachinery/pkg/api/meta"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

// WatcherSpec defines the desired state of Watcher.
type WatcherSpec struct {
	// ServiceInfo describes the service information of the listener
	ServiceInfo Service `json:"serviceInfo"`

	// LabelsToWatch describes the labels that should be watched
	LabelsToWatch map[string]string `json:"labelsToWatch"`

	// ResourceToWatch is the GroupVersionResource of the resource that should be watched.
	ResourceToWatch WatchableGVR `json:"resourceToWatch"`

	// Field describes the subresource that should be watched
	// Value can be one of ("spec", "status")
	Field FieldName `json:"field"`

	// Gateway configures the Istio Gateway for the VirtualService that is created/updated during processing
	// of the Watcher CR.
	Gateway GatewayConfig `json:"gateway"`
}

// WatchableGVR unambiguously identifies the resource that should be watched.
type WatchableGVR struct {
	Group    string `json:"group"`
	Version  string `json:"version"`
	Resource string `json:"resource"`
}

// +kubebuilder:validation:Enum=spec;status;
type FieldName string

const (
	// SpecField represents FieldName spec, which indicates that resource spec will be watched.
	SpecField FieldName = "spec"
	// StatusField represents FieldName status, which indicates that only resource status will be watched.
	StatusField FieldName = "status"
)

// GatewayConfig is used to select an Istio Gateway object in the cluster.
type GatewayConfig struct {
	// LabelSelector allows to select the Gateway using label selectors as defined in the K8s LIST API.
	LabelSelector apimetav1.LabelSelector `json:"selector"`
}

// Service describes the service specification for the corresponding operator container.
type Service struct {
	// Port describes the service port.
	Port int64 `json:"port"`

	// Name describes the service name.
	Name string `json:"name"`

	// Namespace describes the service namespace.
	Namespace string `json:"namespace"`
}

// WatcherStatus defines the observed state of Watcher.
// +kubebuilder:subresource:status
type WatcherStatus struct {
	// State signifies current state of a Watcher.
	// Value can be one of ("Ready", "Processing", "Error", "Deleting", "Warning")
	State shared.State `json:"state"`

	// List of status conditions to indicate the status of a Watcher.
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=type
	Conditions []apimetav1.Condition `json:"conditions"`

	// ObservedGeneration
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status

// Watcher is the Schema for the watchers API.
type Watcher struct {
	apimetav1.TypeMeta   `json:",inline"`
	apimetav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WatcherSpec   `json:"spec,omitempty"`
	Status WatcherStatus `json:"status,omitempty"`
}

func (watcher *Watcher) GetModuleName() string {
	if watcher.Labels == nil {
		return ""
	}
	return watcher.Labels[shared.ManagedBy]
}

// +kubebuilder:object:root=true

// WatcherList contains a list of Watcher.
type WatcherList struct {
	apimetav1.TypeMeta `json:",inline"`
	apimetav1.ListMeta `json:"metadata,omitempty"`
	Items              []Watcher `json:"items"`
}

//nolint:gochecknoinits // registers Watcher CRD on startup
func init() {
	SchemeBuilder.Register(&Watcher{}, &WatcherList{})
}

// DefaultIstioGatewaySelector defines a default label selector for a Gateway to configure a VirtualService
// for the Watcher.
func DefaultIstioGatewaySelector() apimetav1.LabelSelector {
	return apimetav1.LabelSelector{
		MatchLabels: map[string]string{shared.OperatorGroup + shared.Separator + "watcher-gateway": "default"},
	}
}

type WatcherConditionType string

const (
	// WatcherConditionTypeVirtualService represents WatcherConditionType VirtualService.
	WatcherConditionTypeVirtualService WatcherConditionType = "VirtualService"
)

// +kubebuilder:validation:Enum=Ready

type WatcherConditionReason string

const (
	// ReadyConditionReason will be set to `Ready` on all Conditions. If the Condition is actual ready,
	// can be determined by the state.
	ReadyConditionReason WatcherConditionReason = "Ready"
)

type WatcherConditionMessage string

const (
	VirtualServiceConfiguredConditionMessage    WatcherConditionMessage = "VirtualService is configured"
	VirtualServiceNotConfiguredConditionMessage WatcherConditionMessage = "VirtualService is not configured"
)

func (watcher *Watcher) InitializeConditions() {
	watcher.Status.Conditions = []apimetav1.Condition{
		{
			Type:               string(WatcherConditionTypeVirtualService),
			Status:             apimetav1.ConditionUnknown,
			Message:            string(VirtualServiceNotConfiguredConditionMessage),
			Reason:             string(ReadyConditionReason),
			LastTransitionTime: apimetav1.Now(),
		},
	}
}

func (watcher *Watcher) UpdateWatcherConditionStatus(conditionType WatcherConditionType,
	conditionStatus apimetav1.ConditionStatus,
) {
	newCondition := apimetav1.Condition{
		Type:               string(conditionType),
		Status:             conditionStatus,
		Message:            string(VirtualServiceNotConfiguredConditionMessage),
		Reason:             string(ReadyConditionReason),
		LastTransitionTime: apimetav1.Now(),
	}
	switch conditionStatus {
	case apimetav1.ConditionTrue:
		newCondition.Message = string(VirtualServiceConfiguredConditionMessage)
	case apimetav1.ConditionFalse, apimetav1.ConditionUnknown:
		fallthrough
	default:
		newCondition.Message = string(VirtualServiceNotConfiguredConditionMessage)
	}
	meta.SetStatusCondition(&watcher.Status.Conditions, newCondition)
}
