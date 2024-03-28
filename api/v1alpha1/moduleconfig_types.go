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
	"github.com/kyma-project/lifecycle-manager/api/shared"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ModuleConfigSpec defines the desired state of ModuleConfig.
type ModuleConfigSpec struct {
	// Kyma specifies the related Kyma name.
	Kyma string `json:"kyma"`
	// Module specifies the related module name.
	Module string `json:"module"`

	// Resource contains a copy of the related Module CR's manifest.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:XEmbeddedResource
	Resource *unstructured.Unstructured `json:"resource,omitempty"`
}

// ModuleConfigStatus defines the observed state of ModuleConfig.
type ModuleConfigStatus struct {
	// List of status conditions to indicate the status of ModuleConfig.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []apimetav1.Condition `json:"conditions,omitempty"`

	shared.LastOperation `json:"lastOperation,omitempty"`

	// State signifies current state of ModuleConfig.
	// Value can be one of ("Ready", "Processing", "Warning", "Error", "Deleting").
	State shared.State `json:"state,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=".status.state"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ModuleConfig is the Schema for the moduleconfigs API
type ModuleConfig struct {
	apimetav1.TypeMeta   `json:",inline"`
	apimetav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModuleConfigSpec   `json:"spec,omitempty"`
	Status ModuleConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ModuleConfigList contains a list of ModuleConfig
type ModuleConfigList struct {
	apimetav1.TypeMeta `json:",inline"`
	apimetav1.ListMeta `json:"metadata,omitempty"`
	Items              []ModuleConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ModuleConfig{}, &ModuleConfigList{})
}
