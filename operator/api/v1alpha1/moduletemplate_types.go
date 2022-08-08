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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ModuleTemplateSpec defines the desired state of ModuleTemplate.
type ModuleTemplateSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Channel Channel `json:"channel,omitempty"`

	//+kubebuilder:pruning:PreserveUnknownFields
	//+kubebuilder:validation:XEmbeddedResource
	Data unstructured.Unstructured `json:"data,omitempty"`

	Overrides `json:"configSelector,omitempty"`

	//+kubebuilder:pruning:PreserveUnknownFields
	OCMDescriptor runtime.RawExtension `json:"descriptor,omitempty"`

	Target Target `json:"target"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// ModuleTemplate is the Schema for the moduletemplates API.
type ModuleTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ModuleTemplateSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// ModuleTemplateList contains a list of ModuleTemplate.
type ModuleTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ModuleTemplate `json:"items"`
}

// +kubebuilder:validation:Enum=control-plane;remote
type Target string

const (
	TargetRemote       Target = "remote"
	TargetControlPlane Target = "control-plane"
)

//nolint:gochecknoinits
func init() {
	SchemeBuilder.Register(&ModuleTemplate{}, &ModuleTemplateList{})
}
