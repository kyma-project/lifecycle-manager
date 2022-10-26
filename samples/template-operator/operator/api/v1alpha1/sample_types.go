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

	"github.com/kyma-project/module-manager/operator/pkg/types"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=".status.state"

// Sample is the Schema for the samples API
type Sample struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SampleSpec   `json:"spec,omitempty"`
	Status SampleStatus `json:"status,omitempty"`
}

type SampleStatus struct {
	types.Status `json:",inline"`
	// add other fields to status subresource here
}

var _ types.CustomObject = &Sample{}

func (s *Sample) GetStatus() types.Status {
	return s.Status.Status
}

func (s *Sample) SetStatus(status types.Status) {
	s.Status.Status = status
}

func (s *Sample) ComponentName() string {
	return "sample-component-name"
}

type SampleSpec struct {
	// TODO: Implement spec properties here
	ReleaseName string `json:"releaseName,omitempty"`
}

// +kubebuilder:object:root=true

// SampleList contains a list of Sample
type SampleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Sample `json:"items"`
}

func init() { //nolint:gochecknoinits
	SchemeBuilder.Register(&Sample{}, &SampleList{})
}
