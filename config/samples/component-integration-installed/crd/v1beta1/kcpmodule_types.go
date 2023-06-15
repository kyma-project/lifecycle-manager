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

// +groupName=operator.kyma-project.io
package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KCPModuleSpec defines the desired state of KCPModule.
type KCPModuleSpec struct {
	InitKey string `json:"initKey,omitempty"`
	NewKey  string `json:"newKey,omitempty"`
}

type RefTypeMetadata string

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=".status.state"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// KCPModule is the Schema for the moduletemplates API.
type KCPModule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KCPModuleSpec   `json:"spec,omitempty"`
	Status KCPModuleStatus `json:"status,omitempty"`
}

// +kubebuilder:validation:Enum=Processing;Deleting;Ready;Error;Warning
type KCPModuleState string

// ManifestStatus defines the observed state of Manifest.
type KCPModuleStatus struct {
	// +kubebuilder:validation:Enum=Ready;Processing;Error;Deleting;Warning
	State KCPModuleState `json:"state"`
}

//+kubebuilder:object:root=true

// KCPModuleList contains a list of KCPModule.
type KCPModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KCPModule `json:"items"`
}
