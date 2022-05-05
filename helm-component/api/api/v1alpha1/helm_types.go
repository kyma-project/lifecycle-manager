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

// HelmSpec defines the desired state of Helm
type HelmSpec struct {
	RepoName         string `json:"repoName,omitempty"`
	Url              string `json:"url,omitempty"`
	ChartName        string `json:"chartName,omitempty"`
	ReleaseNamespace string `json:"releaseNamespace,omitempty"`
	ReleaseName      string `json:"releaseName,omitempty"`
	Create           string `json:"create,omitempty"`
}

// +kubebuilder:validation:Enum=Processing;Deleting;Ready;Error
type HelmState string

// Valid Helm States
const (
	// HelmStateReady signifies Helm is ready
	HelmStateReady HelmState = "Ready"

	// HelmStateProcessing signifies Helm is reconciling
	HelmStateProcessing HelmState = "Processing"

	// HelmStateError signifies an error for Helm
	HelmStateError HelmState = "Error"

	// HelmStateDeleting signifies Helm is being deleted
	HelmStateDeleting HelmState = "Deleting"
)

// HelmStatus defines the observed state of Helm
type HelmStatus struct {
	State HelmState `json:"state,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=".status.state"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Helm is the Schema for the helms API
type Helm struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HelmSpec   `json:"spec,omitempty"`
	Status HelmStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HelmList contains a list of Helm
type HelmList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Helm `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Helm{}, &HelmList{})
}
