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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=".status.state"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:deprecatedversion:warning="kyma-project.io/v1beta1 Manifest is deprecated. Use v1beta2 instead."
// +kubebuilder:unservedversion

// Manifest is the Schema for the manifests API.
type Manifest struct {
	apimetav1.TypeMeta   `json:",inline"`
	apimetav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManifestSpec `json:"spec,omitempty"`
	Status Status       `json:"status,omitempty"`
}

// ManifestSpec defines the desired state of Manifest.
type ManifestSpec struct {
	// +kubebuilder:default:=CreateAndDelete
	CustomResourcePolicy `json:"customResourcePolicy,omitempty"`

	// Remote indicates if Manifest should be installed on a remote cluster
	Remote bool `json:"remote"`

	// Version specifies current Resource version
	// +optional
	Version string `json:"version,omitempty"`

	// Config specifies OCI image configuration for Manifest
	Config *ImageSpec `json:"config,omitempty"`

	// Install specifies a list of installations for Manifest
	Install InstallInfo `json:"install"`

	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:XEmbeddedResource
	// +nullable
	// Resource specifies a resource to be watched for state updates
	Resource *unstructured.Unstructured `json:"resource,omitempty"`
}

// InstallInfo defines installation information.
type InstallInfo struct {
	// Source in the ImageSpec format
	// +kubebuilder:pruning:PreserveUnknownFields
	Source machineryruntime.RawExtension `json:"source"`

	// Name specifies a unique install name for Manifest
	Name string `json:"name"`
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

	// Deprecated: Field will be removed soon and is not supported anymore.
	CredSecretSelector *apimetav1.LabelSelector `json:"credSecretSelector,omitempty"`
}

type RefTypeMetadata string

const (
	OciRefType RefTypeMetadata = "oci-ref"
	OciDirType RefTypeMetadata = "oci-dir"
)

// +kubebuilder:object:root=true

// ManifestList contains a list of Manifest.
type ManifestList struct {
	apimetav1.TypeMeta `json:",inline"`
	apimetav1.ListMeta `json:"metadata,omitempty"`

	Items []Manifest `json:"items"`
}

//nolint:gochecknoinits // registers Kyma CRD on startup
func init() {
	SchemeBuilder.Register(&Manifest{}, &ManifestList{})
}
