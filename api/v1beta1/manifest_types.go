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

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=".status.state"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:deprecatedversion:warning="kyma-project.io/v1beta1 Manifest is deprecated. Use v1beta2 instead."
// +kubebuilder:storageversion

// Manifest is the Schema for the manifests API.
type Manifest struct {
	apimetav1.TypeMeta   `json:",inline"`
	apimetav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   v1beta2.ManifestSpec `json:"spec,omitempty"`
	Status shared.Status        `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ManifestList contains a list of Manifest.
type ManifestList struct {
	apimetav1.TypeMeta `json:",inline"`
	apimetav1.ListMeta `json:"metadata,omitempty"`
	Items              []Manifest `json:"items"`
}
