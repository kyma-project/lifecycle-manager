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
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:deprecatedversion:warning="kyma-project.io/v1beta1 Watcher is deprecated. Use v1beta2 instead."
// +kubebuilder:unservedversion

// Watcher is the Schema for the watchers API.
type Watcher struct {
	apimetav1.TypeMeta `json:",inline"`

	// +kubebuilder:validation:Optional
	apimetav1.ObjectMeta `json:"metadata"`

	// +kubebuilder:validation:Optional
	Spec WatcherSpec `json:"spec"`

	Status WatcherStatus `json:"status,omitempty"`
}

// WatcherSpec defines the desired state of Watcher.
type WatcherSpec struct {
	// ServiceInfo describes the service information of the listener
	ServiceInfo Service `json:"serviceInfo"`

	// LabelsToWatch describes the labels that should be watched
	// +optional
	LabelsToWatch map[string]string `json:"labelsToWatch,omitempty"`

	// ResourceToWatch is the GroupVersionResource of the resource that should be watched.
	ResourceToWatch WatchableGVR `json:"resourceToWatch"`

	// Field describes the subresource that should be watched
	// Value can be one of ("spec", "status")
	Field FieldName `json:"field"`

	// Gateway configures the Istio Gateway for the VirtualService that is created/updated during processing
	// of the Watcher CR.
	Gateway GatewayConfig `json:"gateway"`
}

type WatchableGVR struct {
	Group    string `json:"group"`
	Version  string `json:"version"`
	Resource string `json:"resource"`
}

// +kubebuilder:validation:Enum=spec;status;
type FieldName string

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
type WatcherStatus struct {
	// State signifies current state of a Watcher.
	// Value can be one of ("Ready", "Processing", "Error", "Deleting", "Warning")
	State State `json:"state"`

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

// WatcherList contains a list of Watcher.
type WatcherList struct {
	apimetav1.TypeMeta `json:",inline"`

	// +kubebuilder:validation:Optional
	apimetav1.ListMeta `json:"metadata"`
	Items              []Watcher `json:"items"`
}

//nolint:gochecknoinits // registers Watcher CRD on startup
func init() {
	SchemeBuilder.Register(&Watcher{}, &WatcherList{})
}
