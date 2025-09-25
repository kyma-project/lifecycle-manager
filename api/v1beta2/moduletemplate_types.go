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
	"strings"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

// ModuleTemplate is a representation of a Template used for creating Module Instances within the Module Lifecycle.
// It is generally loosely defined within the Kubernetes Specification, however it has a strict enforcement of
// OCM guidelines as it serves an active role in maintaining a list of available Modules within a cluster.
//
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
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
	// Deprecated: This field is deprecated and will be removed in a future release.
	// +optional
	// +kubebuilder:deprecatedversion
	// +kubebuilder:validation:Pattern:=`^$|^[a-z]{3,}$`
	// +kubebuilder:validation:MaxLength:=32
	Channel string `json:"channel"`

	// Version identifies the version of the Module. Can be empty, or a semantic version.
	// +optional
	// +kubebuilder:validation:Pattern:=`^((0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[a-zA-Z-][0-9a-zA-Z-]*)?)?$`
	// +kubebuilder:validation:MaxLength:=32
	Version string `json:"version"`

	// ModuleName is the name of the Module. Can be empty.
	// +optional
	// +kubebuilder:validation:Pattern:=`^([a-z]{3,}(-[a-z]{3,})*)?$`
	// +kubebuilder:validation:MaxLength:=64
	ModuleName string `json:"moduleName"`

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
	// to correctly initialize a module (e.g. Manifests, References to Binaries and/or configuration)
	// Name more information on Component Descriptors, see
	// https://github.com/open-component-model/ocm
	//
	// It is translated inside the Lifecycle of the Cluster and will be used by downstream controllers
	// to bootstrap and manage the module. This part is also propagated for every change of the template.
	// This means for upgrades of the Descriptor, downstream controllers will also update the dependant modules
	// (e.g. by updating the controller binary linked in a chart referenced in the descriptor)
	//
	// NOTE: Only Raw Rendering is Supported for the layers. So previously used "config" layers for the helm
	// charts and kustomize renderers are deprecated and ignored.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	Descriptor machineryruntime.RawExtension `json:"descriptor"`

	// CustomStateCheck is deprecated.
	CustomStateCheck []*CustomStateCheck `json:"customStateCheck,omitempty"`

	// Resources is a list of additional resources of the module that can be fetched, e.g., the raw manifest.
	// +optional
	// +listType=map
	// +listMapKey=name
	Resources []Resource `json:"resources,omitempty"`

	// Info contains metadata about the module.
	// +optional
	Info *ModuleInfo `json:"info,omitempty"`

	// AssociatedResources is a list of module related resources that usually must be cleaned when uninstalling a module. Informational purpose only.
	// +optional
	AssociatedResources []apimetav1.GroupVersionKind `json:"associatedResources,omitempty"`

	// Manager contains information for identifying a module's resource that can be used as indicator for the installation readiness of the module. Typically, this is the manager Deployment of the module. In exceptional cases, it may also be another resource.
	// +optional
	Manager *Manager `json:"manager,omitempty"`

	// RequiresDowntime indicates whether the module requires downtime in support of maintenance windows during module upgrades.
	// +optional
	RequiresDowntime bool `json:"requiresDowntime"`
}

// Manager defines the structure for the manager field in ModuleTemplateSpec.
type Manager struct {
	apimetav1.GroupVersionKind `json:",inline"`

	// Namespace is the namespace of the manager. It is optional.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name is the name of the manager.
	Name string `json:"name"`
}

type ModuleInfo struct {
	// Repository is the link to the repository of the module.
	Repository string `json:"repository"`

	// Documentation is the link to the documentation of the module.
	Documentation string `json:"documentation"`

	// Icons is a list of icons of the module.
	// +optional
	// +listType=map
	// +listMapKey=name
	Icons []ModuleIcon `json:"icons,omitempty"`
}

type ModuleIcon struct {
	// Name is the name of the icon.
	Name string `json:"name"`

	// Link is the link to the icon.
	Link string `json:"link"`
}

type CustomStateCheck struct {
	// JSONPath specifies the JSON path to the state variable in the Module CR
	JSONPath string `json:"jsonPath" yaml:"jsonPath"`

	// Value is the value at the JSONPath for which the Module CR state should map with MappedState
	Value string `json:"value" yaml:"value"`

	// MappedState is the Kyma CR State
	MappedState shared.State `json:"mappedState" yaml:"mappedState"`
}

// +kubebuilder:object:root=true

// ModuleTemplateList contains a list of ModuleTemplate.
type ModuleTemplateList struct {
	apimetav1.TypeMeta `json:",inline"`
	apimetav1.ListMeta `json:"metadata,omitempty"`

	Items []ModuleTemplate `json:"items"`
}

type Resource struct {
	// Name is the name of the resource.
	Name string `json:"name"`
	// Link is the URL to the resource.
	// +kubebuilder:validation:Format=uri
	Link string `json:"link"`
}

//nolint:gochecknoinits // registers ModuleTemplate CRD on startup
func init() {
	SchemeBuilder.Register(&ModuleTemplate{}, &ModuleTemplateList{})
}

func (m *ModuleTemplate) GetVersion() string {
	return m.Spec.Version
}

func (m *ModuleTemplate) IsMandatory() bool {
	return m.Spec.Mandatory
}

func (m *ModuleTemplate) GetModuleName() string {
	return m.Spec.ModuleName
}

// https://github.com/kyma-project/lifecycle-manager/issues/2096
// Remove this function after the migration to the new ModuleTemplate format is completed.
func (m *ModuleTemplate) IsInternal() bool {
	if isInternal, found := m.Labels[shared.InternalLabel]; found {
		return strings.ToLower(isInternal) == shared.EnableLabelValue
	}
	return false
}

// https://github.com/kyma-project/lifecycle-manager/issues/2096
// Remove this function after the migration to the new ModuleTemplate format is completed.
func (m *ModuleTemplate) IsBeta() bool {
	if isBeta, found := m.Labels[shared.BetaLabel]; found {
		return strings.ToLower(isBeta) == shared.EnableLabelValue
	}
	return false
}
