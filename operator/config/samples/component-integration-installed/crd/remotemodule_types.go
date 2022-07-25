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

//+groupName=component.kyma-project.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RemoteModuleSpec defines the desired state of RemoteModule.
type RemoteModuleSpec struct {
	// Config specifies OCI image configuration
	// +optional
	Config ImageSpec `json:"config"`
}

//+genclient
//+kubebuilder:object:root=true
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// RemoteModule is the Schema for the moduletemplates API.
type RemoteModule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RemoteModuleSpec         `json:"spec,omitempty"`
	Status ControlPlaneModuleStatus `json:"status,omitempty"`
}

// +kubebuilder:validation:Enum=Processing;Deleting;Ready;Error
type RemoteModuleState string

const (
	RemoteModuleStateReady RemoteModuleState = "Ready"

	RemoteModuleStateProcessing RemoteModuleState = "Processing"

	RemoteModuleStateError RemoteModuleState = "Error"

	RemoteModuleStateDeleting RemoteModuleState = "Deleting"
)

// ManifestStatus defines the observed state of Manifest.
type RemoteModuleStatus struct {
	State RemoteModuleState `json:"state,omitempty"`
}

//+kubebuilder:object:root=true

// ControlPlaneModuleList contains a list of RemoteModule.
type RemoteModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RemoteModule `json:"items"`
}
