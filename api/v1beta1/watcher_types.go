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

package v1beta1

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WatcherSpec defines the desired state of Watcher.
type WatcherSpec struct {
	// ServiceInfo describes the service information of the operator
	ServiceInfo Service `json:"serviceInfo"`

	// LabelsToWatch describes the labels that should be watched
	LabelsToWatch map[string]string `json:"labelsToWatch"`

	// Field describes the subresource that should be watched
	// Value can be one of ("spec", "status")
	Field FieldName `json:"field"`

	// Gateway configures the Istio Gateway for the VirtualService that is created/updated during processing
	// of the Watcher CR.
	Gateway GatewayConfig `json:"gateway"`
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
	LabelSelector metav1.LabelSelector `json:"selector"`
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

// +kubebuilder:validation:Enum=Processing;Deleting;Ready;Error;""
type WatcherState string

// Valid Watcher States.
const (
	// WatcherStateReady signifies Watcher is ready and has been installed successfully.
	WatcherStateReady WatcherState = "Ready"

	// WatcherStateProcessing signifies Watcher is reconciling and is in the process of installation.
	WatcherStateProcessing WatcherState = "Processing"

	// WatcherStateError signifies an error for Watcher. This signifies that the Installation
	// process encountered an error.
	WatcherStateError WatcherState = "Error"

	// WatcherStateDeleting signifies Watcher is being deleted.
	WatcherStateDeleting WatcherState = "Deleting"
)

// WatcherStatus defines the observed state of Watcher.
// +kubebuilder:subresource:status
type WatcherStatus struct {
	// State signifies current state of a Watcher.
	// Value can be one of ("Ready", "Processing", "Error", "Deleting")
	State WatcherState `json:"state"`

	// List of status conditions to indicate the status of a Watcher.
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions"`

	// ObservedGeneration
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration"`
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

// Valid ConditionStatus.
const (
	// ConditionStatusTrue signifies ConditionStatus true.
	ConditionStatusTrue metav1.ConditionStatus = "True"

	// ConditionStatusFalse signifies ConditionStatus false.
	ConditionStatusFalse metav1.ConditionStatus = "False"

	// ConditionStatusUnknown signifies ConditionStatus unknown.
	ConditionStatusUnknown metav1.ConditionStatus = "Unknown"
)

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:storageversion
// +kubebuilder:subresource:status

// Watcher is the Schema for the watchers API.
type Watcher struct {
	metav1.TypeMeta `json:",inline"`

	// +kubebuilder:validation:Optional
	metav1.ObjectMeta `json:"metadata"`

	// +kubebuilder:validation:Optional
	Spec WatcherSpec `json:"spec"`

	Status WatcherStatus `json:"status,omitempty"`
}

func (w *Watcher) GetModuleName() string {
	if w.Labels == nil {
		return ""
	}
	return w.Labels[ManagedBy]
}

func (w *Watcher) InitializeConditions() {
	w.Status.Conditions = []metav1.Condition{{
		Type:               string(WatcherConditionTypeVirtualService),
		Status:             ConditionStatusUnknown,
		Message:            string(VirtualServiceNotConfiguredConditionMessage),
		Reason:             string(ReadyConditionReason),
		LastTransitionTime: metav1.Now(),
	}}
}

func (w *Watcher) UpdateWatcherConditionStatus(conditionType WatcherConditionType,
	conditionStatus metav1.ConditionStatus,
) {
	newCondition := metav1.Condition{
		Type:               string(conditionType),
		Status:             conditionStatus,
		Message:            string(VirtualServiceNotConfiguredConditionMessage),
		Reason:             string(ReadyConditionReason),
		LastTransitionTime: metav1.Now(),
	}
	switch conditionStatus {
	case ConditionStatusTrue:
		newCondition.Message = string(VirtualServiceConfiguredConditionMessage)
	case ConditionStatusFalse, ConditionStatusUnknown:
		fallthrough
	default:
		newCondition.Message = string(VirtualServiceNotConfiguredConditionMessage)
	}
	meta.SetStatusCondition(&w.Status.Conditions, newCondition)
}

//+kubebuilder:object:root=true

// WatcherList contains a list of Watcher.
type WatcherList struct {
	metav1.TypeMeta `json:",inline"`

	// +kubebuilder:validation:Optional
	metav1.ListMeta `json:"metadata"`
	Items           []Watcher `json:"items"`
}

func init() { //nolint:gochecknoinits
	SchemeBuilder.Register(&Watcher{}, &WatcherList{})
}

// DefaultIstioGatewaySelector defines a default label selector for a Gateway to configure a VirtualService
// for the Watcher.
func DefaultIstioGatewaySelector() metav1.LabelSelector {
	return metav1.LabelSelector{
		MatchLabels: map[string]string{OperatorPrefix + Separator + "watcher-gateway": "default"},
	}
}
