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
	"fmt"

	declarative "github.com/kyma-project/lifecycle-manager/pkg/declarative/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const ManifestKind = "Manifest"

// InstallInfo defines installation information.
type InstallInfo struct {
	// Source can either be described as ImageSpec, HelmChartSpec or KustomizeSpec
	//+kubebuilder:pruning:PreserveUnknownFields
	Source runtime.RawExtension `json:"source"`

	// Name specifies a unique install name for Manifest
	Name string `json:"name"`
}

func (i InstallInfo) Raw() []byte {
	return i.Source.Raw
}

// ManifestSpec defines the specification of Manifest.
type ManifestSpec struct {
	// Remote indicates if Manifest should be installed on a remote cluster
	Remote bool `json:"remote"`

	// Config specifies OCI image configuration for Manifest
	Config ImageSpec `json:"config,omitempty"`

	// Installs specifies a list of installations for Manifest
	Installs []InstallInfo `json:"installs"`

	//+kubebuilder:pruning:PreserveUnknownFields
	//+kubebuilder:validation:XEmbeddedResource
	//+nullable
	// Resource specifies a resource to be watched for state updates
	Resource *unstructured.Unstructured `json:"resource,omitempty"`

	// CRDs specifies the custom resource definitions' ImageSpec
	CRDs ImageSpec `json:"crds,omitempty"`
}

// ManifestStatus defines the observed state of Manifest.
type ManifestStatus declarative.Status

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=".status.state"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:deprecatedversion:warning="kyma-project.io/v1alpha1 Manifest is deprecated. Use v1beta1 instead."

// Manifest is the Schema for the manifests API.
type Manifest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec specifies the content and configuration for Manifest
	Spec ManifestSpec `json:"spec,omitempty"`

	// Status signifies the current status of the Manifest
	// +kubebuilder:validation:Optional
	Status ManifestStatus `json:"status,omitempty"`
}

func (m *Manifest) ComponentName() string {
	return fmt.Sprintf("manifest-%s", m.Name)
}

func (m *Manifest) GetStatus() declarative.Status {
	return declarative.Status(m.Status)
}

func (m *Manifest) SetStatus(status declarative.Status) {
	m.Status = ManifestStatus(status)
}

//+kubebuilder:object:root=true

// ManifestList contains a list of Manifest.
type ManifestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Manifest `json:"items"`
}

//nolint:gochecknoinits
func init() {
	SchemeBuilder.Register(&Manifest{}, &ManifestList{})
}
