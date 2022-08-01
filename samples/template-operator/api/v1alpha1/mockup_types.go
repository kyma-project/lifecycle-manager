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

// MockupSpec defines the desired state of Mockup
type MockupSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Mockup. Edit mockup_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// +kubebuilder:validation:Enum=Processing;Deleting;Ready;Error
type MockupState string

// Valid Mockup States.
const (
	// MockupStateReady signifies Mockup is ready and has been installed successfully.
	MockupStateReady MockupState = "Ready"

	// MockupStateProcessing signifies Mockup is reconciling and is in the process of installation. Processing can also
	// signal that the Installation previously encountered an error and is now recovering.
	MockupStateProcessing MockupState = "Processing"

	// MockupStateError signifies an error for Mockup. This signifies that the Installation process encountered an error.
	// Contrary to Processing, it can be expected that this state should change on the next retry.
	MockupStateError MockupState = "Error"

	// MockupStateDeleting signifies Mockup is being deleted. This is the state that is used when a deletionTimestamp
	// was detected and Finalizers are picked up.
	MockupStateDeleting MockupState = "Deleting"
)

// MockupStatus defines the observed state of Mockup
// +kubebuilder:subresource:status
type MockupStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// State signifies current state of Mockup.
	// Value can be one of ("Ready", "Processing", "Error", "Deleting").
	State MockupState `json:"state,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Mockup is the Schema for the mockups API
type Mockup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MockupSpec   `json:"spec,omitempty"`
	Status MockupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MockupList contains a list of Mockup
type MockupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Mockup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Mockup{}, &MockupList{})
}
