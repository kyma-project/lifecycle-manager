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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SampleSpec defines the desired state of Sample
type SampleSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Sample. Edit sample_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// +kubebuilder:validation:Enum=Processing;Deleting;Ready;Error
type SampleState string

// Valid Sample States.
const (
	// SampleStateReady signifies Sample is ready and has been installed successfully.
	SampleStateReady SampleState = "Ready"

	// SampleStateProcessing signifies Sample is reconciling and is in the process of installation. Processing can also
	// signal that the Installation previously encountered an error and is now recovering.
	SampleStateProcessing SampleState = "Processing"

	// SampleStateError signifies an error for Sample. This signifies that the Installation process encountered an error.
	// Contrary to Processing, it can be expected that this state should change on the next retry.
	SampleStateError SampleState = "Error"

	// SampleStateDeleting signifies Sample is being deleted. This is the state that is used when a deletionTimestamp
	// was detected and Finalizers are picked up.
	SampleStateDeleting SampleState = "Deleting"
)

// SampleStatus defines the observed state of Sample
// +kubebuilder:subresource:status
type SampleStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// State signifies current state of Sample.
	// Value can be one of ("Ready", "Processing", "Error", "Deleting").
	State SampleState `json:"state,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Sample is the Schema for the samples API
type Sample struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SampleSpec   `json:"spec,omitempty"`
	Status SampleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SampleList contains a list of Sample
type SampleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Sample `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Sample{}, &SampleList{})
}
