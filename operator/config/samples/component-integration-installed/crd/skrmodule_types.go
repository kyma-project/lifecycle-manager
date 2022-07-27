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

// SKRModuleSpec defines the desired state of SKRModule.
type SKRModuleSpec struct {
	InitKey string `json:"initKey,omitempty"`
}

//+genclient
//+kubebuilder:object:root=true
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=".status.state"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// SKRModule is the Schema for the moduletemplates API.
type SKRModule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SKRModuleSpec   `json:"spec,omitempty"`
	Status SKRModuleStatus `json:"status,omitempty"`
}

// +kubebuilder:validation:Enum=Processing;Deleting;Ready;Error
type SKRModuleState string

// SKRModuleStatus defines the observed state of Manifest.
type SKRModuleStatus struct {
	State SKRModuleState `json:"state,omitempty"`
}

//+kubebuilder:object:root=true

// SKRModuleList contains a list of SKRModule.
type SKRModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SKRModule `json:"items"`
}
