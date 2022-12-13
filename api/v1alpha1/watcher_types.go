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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
	// +kubebuilder:validation:Optional
	Gateway *GatewayConfig `json:"gateway,omitempty"`
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
	// NamespacedName takes precedence over LabelSelector if configured. Format to use: "namespaceName/gatewayName"
	// TODO: Add validation: <name><slash><namespace>
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength=3
	// +kubebuilder:validation:MaxLength=127
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?[/][a-z]([-a-z0-9]*[a-z0-9])?$`
	NamespacedName string `json:"namespacedName,omitempty"`

	// LabelSelector allows to select the Gateway using label selectors as defined in the K8s LIST API.
	// Ignored if NamespacedName is set.
	// +kubebuilder:validation:Optional
	LabelSelector *metav1.LabelSelector `json:"selector,omitempty"`
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

// +kubebuilder:validation:Enum=Processing;Deleting;Ready;Error
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
type WatcherStatus struct {
	// State signifies current state of a Watcher.
	// Value can be one of ("Ready", "Processing", "Error", "Deleting")
	State WatcherState `json:"state"`

	// List of status conditions to indicate the status of a Watcher.
	// +kubebuilder:validation:Optional
	Conditions []WatcherCondition `json:"conditions"`

	// ObservedGeneration
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration"`
}

// WatcherCondition describes condition information for Watcher.
type WatcherCondition struct {
	// Type is used to reflect what type of condition we are dealing with.
	// Most commonly WatcherConditionTypeReady it is used as extension marker in the future.
	Type WatcherConditionType `json:"type"`

	// Status of the Watcher Condition.
	// Value can be one of ("True", "False", "Unknown").
	Status WatcherConditionStatus `json:"status"`

	// Human-readable message indicating details about the last status transition.
	// +kubebuilder:validation:Optional
	Message string `json:"message"`

	// Machine-readable text indicating the reason for the condition's last transition.
	// +kubebuilder:validation:Optional
	Reason string `json:"reason"`

	// Timestamp for when Watcher last transitioned from one status to another.
	// +kubebuilder:validation:Optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime"`
}

// +kubebuilder:validation:Enum=Ready
type WatcherConditionType string

const (
	// WatcherConditionTypeReady represents WatcherConditionType Ready,
	// meaning as soon as its true we will reconcile Watcher into WatcherStateReady.
	WatcherConditionTypeReady WatcherConditionType = "Ready"
)

// +kubebuilder:validation:Enum=True;False;Unknown;
type WatcherConditionStatus string

// Valid WatcherConditionStatus.
const (
	// ConditionStatusTrue signifies WatcherConditionStatus true.
	ConditionStatusTrue WatcherConditionStatus = "True"

	// ConditionStatusFalse signifies WatcherConditionStatus false.
	ConditionStatusFalse WatcherConditionStatus = "False"

	// ConditionStatusUnknown signifies WatcherConditionStatus unknown.
	ConditionStatusUnknown WatcherConditionStatus = "Unknown"
)

//+kubebuilder:object:root=true
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Watcher is the Schema for the watchers API.
type Watcher struct {
	metav1.TypeMeta `json:",inline"`

	// +kubebuilder:validation:Optional
	metav1.ObjectMeta `json:"metadata"`

	// +kubebuilder:validation:Optional
	Spec WatcherSpec `json:"spec"`
}

func (w *Watcher) GetModuleName() string {
	if w.Labels == nil {
		return ""
	}
	return w.Labels[ManagedBy]
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
func DefaultIstioGatewaySelector() *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{OperatorPrefix + Separator + "watcher-gateway": "default"},
	}
}
