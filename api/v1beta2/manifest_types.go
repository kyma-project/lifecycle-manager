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

package v1beta2

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	ManifestKind         = "Manifest"
	RawManifestLayerName = "raw-manifest"
)

// InstallInfo defines installation information.
type InstallInfo struct {
	// Source in the ImageSpec format
	//+kubebuilder:pruning:PreserveUnknownFields
	Source runtime.RawExtension `json:"source"`

	// Name specifies a unique install name for Manifest
	Name string `json:"name"`
}

func (i InstallInfo) Raw() []byte {
	return i.Source.Raw
}

// ManifestSpec defines the desired state of Manifest.
type ManifestSpec struct {
	// Remote indicates if Manifest should be installed on a remote cluster
	Remote bool `json:"remote"`

	// Config specifies OCI image configuration for Manifest
	Config *ImageSpec `json:"config,omitempty"`

	// Install specifies a list of installations for Manifest
	Install InstallInfo `json:"install"`

	//+kubebuilder:pruning:PreserveUnknownFields
	//+kubebuilder:validation:XEmbeddedResource
	//+nullable
	// Resource specifies a resource to be watched for state updates
	Resource *unstructured.Unstructured `json:"resource,omitempty"`
}

// ImageSpec defines OCI Image specifications.
// +k8s:deepcopy-gen=true
type ImageSpec struct {
	// Repo defines the Image repo
	Repo string `json:"repo,omitempty"`

	// Name defines the Image name
	Name string `json:"name,omitempty"`

	// Ref is either a sha value, tag or version
	Ref string `json:"ref,omitempty"`

	// Type specifies the type of installation specification
	// that could be provided as part of a custom resource.
	// This time is used in codec to successfully decode from raw extensions.
	// +kubebuilder:validation:Enum=helm-chart;oci-ref;"kustomize";""
	Type RefTypeMetadata `json:"type,omitempty"`

	// CredSecretSelector is an optional field, for OCI image saved in private registry,
	// use it to indicate the secret which contains registry credentials,
	// must exist in the namespace same as manifest
	CredSecretSelector *metav1.LabelSelector `json:"credSecretSelector,omitempty"`
}

type RefTypeMetadata string

const (
	OciRefType RefTypeMetadata = "oci-ref"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=".status.state"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Manifest is the Schema for the manifests API.
type Manifest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManifestSpec  `json:"spec,omitempty"`
	Status shared.Status `json:"status,omitempty"`
}

func (manifest *Manifest) GetStatus() shared.Status {
	return manifest.Status
}

func (manifest *Manifest) SetStatus(status shared.Status) {
	manifest.Status = status
}

//+kubebuilder:object:root=true

// ManifestList contains a list of Manifest.
type ManifestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Manifest `json:"items"`
}

//nolint:gochecknoinits
func init() {
	SchemeBuilder.Register(&Manifest{}, &ManifestList{})
}
