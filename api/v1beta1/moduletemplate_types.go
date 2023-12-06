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

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

// ModuleTemplate is a representation of a Template used for creating Module Instances within the Module Lifecycle.
// It is generally loosely defined within the Kubernetes Specification, however it has a strict enforcement of
// OCM guidelines as it serves an active role in maintaining a list of available Modules within a cluster.
//
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:deprecatedversion:warning="kyma-project.io/v1beta1 ModuleTemplate is deprecated. Use v1beta2 instead."
// +kubebuilder:storageversion

type ModuleTemplate struct {
	apimetav1.TypeMeta   `json:",inline"`
	apimetav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ModuleTemplateSpec `json:"spec,omitempty"`
}

// ModuleTemplateSpec defines the desired state of ModuleTemplate.
type ModuleTemplateSpec struct {
	// Channel is the targeted channel of the ModuleTemplate. It will be used to directly assign a Template
	// to a target channel. It has to be provided at any given time.
	// +kubebuilder:validation:Pattern:=^[a-z]+$
	// +kubebuilder:validation:MaxLength:=32
	// +kubebuilder:validation:MinLength:=3
	Channel string `json:"channel"`

	// Mandatory indicates whether the module is mandatory. It is used to enforce the installation of the module with
	// its configuration in all runtime clusters.
	// +optional
	Mandatory bool `json:"mandatory"`

	// Data is the default set of attributes that are used to generate the Module. It contains a default set of values
	// for a given channel, and is thus different from default values allocated during struct parsing of the Module.
	// While Data can change after the initial creation of ModuleTemplate, it is not expected to be propagated to
	// downstream modules as it is considered a set of default values. This means that an update of the data block
	// will only propagate to new Modules created form ModuleTemplate, not any existing Module.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:XEmbeddedResource
	Data *unstructured.Unstructured `json:"data,omitempty"`

	// The Descriptor is the Open Component Model Descriptor of a Module, containing all relevant information
	// to correctly initialize a module (e.g. Charts, Manifests, References to Binaries and/or configuration)
	// Name more information on Component Descriptors, see
	// https://github.com/open-component-model/ocm
	//
	// It is translated inside the Lifecycle of the Cluster and will be used by downstream controllers
	// to bootstrap and manage the module. This part is also propagated for every change of the template.
	// This means for upgrades of the Descriptor, downstream controllers will also update the dependant modules
	// (e.g. by updating the controller binary linked in a chart referenced in the descriptor)
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	Descriptor machineryruntime.RawExtension `json:"descriptor"`

	// Target describes where the Module should later on be installed if parsed correctly. It is used as installation
	// hint by downstream controllers to determine which client implementation to use for working with the Module
	Target Target `json:"target"`

	CustomStateCheck []*v1beta2.CustomStateCheck `json:"customStateCheck,omitempty"`
}

// +kubebuilder:object:root=true

// ModuleTemplateList contains a list of ModuleTemplate.
type ModuleTemplateList struct {
	apimetav1.TypeMeta `json:",inline"`
	apimetav1.ListMeta `json:"metadata,omitempty"`
	Items              []ModuleTemplate `json:"items"`
}

// Target serves as a potential Installation Hint for the Controller to determine which Client to use for installation.
// +kubebuilder:validation:Enum=control-plane;remote
type Target string

const (
	TargetRemote       Target = "remote"
	TargetControlPlane Target = "control-plane"
)

//nolint:gochecknoinits
func init() {
	SchemeBuilder.Register(&ModuleTemplate{}, &ModuleTemplateList{}, &v1beta2.Descriptor{})
}
