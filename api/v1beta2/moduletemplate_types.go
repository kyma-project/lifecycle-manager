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
	"errors"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

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

// +k8s:deepcopy-gen=false
type Descriptor struct {
	*compdesc.ComponentDescriptor
}

func (d *Descriptor) SetGroupVersionKind(kind schema.GroupVersionKind) {
	d.Version = kind.Version
}

func (d *Descriptor) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   "ocm.kyma-project.io",
		Version: d.Metadata.ConfiguredVersion,
		Kind:    "Descriptor",
	}
}

func (d *Descriptor) GetObjectKind() schema.ObjectKind {
	return d
}

func (d *Descriptor) DeepCopyObject() machineryruntime.Object {
	return &Descriptor{ComponentDescriptor: d.Copy()}
}

// ModuleTemplateSpec defines the desired state of ModuleTemplate.
type ModuleTemplateSpec struct {
	// Channel is the targeted channel of the ModuleTemplate. It will be used to directly assign a Template
	// to a target channel. It has to be provided at any given time.
	// +kubebuilder:validation:Pattern:=^[a-z]+$
	// +kubebuilder:validation:MaxLength:=32
	// +kubebuilder:validation:MinLength:=3
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
	Items              []ModuleTemplate `json:"items"`
}

//nolint:gochecknoinits // registers ModuleTemplate CRD on startup
func init() {
	SchemeBuilder.Register(&ModuleTemplate{}, &ModuleTemplateList{}, &Descriptor{})
}

func (m *ModuleTemplate) SyncEnabled(betaEnabled, internalEnabled bool) bool {
	if m.syncDisabled() {
		return false
	}

	if m.IsBeta() && !betaEnabled {
		return false
	}

	if m.IsInternal() && !internalEnabled {
		return false
	}

	if m.IsMandatory() {
		return false
	}

	return true
}

func (m *ModuleTemplate) syncDisabled() bool {
	if isSync, found := m.Labels[shared.SyncLabel]; found {
		return strings.ToLower(isSync) == shared.DisableLabelValue
	}
	return false
}

func (m *ModuleTemplate) IsInternal() bool {
	if isInternal, found := m.Labels[shared.InternalLabel]; found {
		return strings.ToLower(isInternal) == shared.EnableLabelValue
	}
	return false
}

var ErrInvalidVersion = errors.New("can't find valid semantic version")

// getVersionLegacy() returns the version of the ModuleTemplate from the annotation on the object.
// Remove once shared.ModuleVersionAnnotation is removed
func (m *ModuleTemplate) getVersionLegacy() (string, error) {
	if m.Annotations != nil {
		moduleVersion, found := m.Annotations[shared.ModuleVersionAnnotation]
		if found {
			return moduleVersion, nil
		}
	}
	return "", ErrInvalidVersion
}

// GetVersion returns the declared version of the ModuleTemplate from it's Spec.
func (m *ModuleTemplate) GetVersion() (*semver.Version, error) {
	var versionValue string
	var err error

	if m.Spec.Version == "" {
		versionValue, err = m.getVersionLegacy()
		if err != nil {
			return nil, err
		}
	} else {
		versionValue = m.Spec.Version

	}

	version, err := semver.NewVersion(versionValue)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidVersion, err.Error())
	}
	return version, nil
}

func (m *ModuleTemplate) IsBeta() bool {
	if isBeta, found := m.Labels[shared.BetaLabel]; found {
		return strings.ToLower(isBeta) == shared.EnableLabelValue
	}
	return false
}

func (m *ModuleTemplate) IsMandatory() bool {
	return m.Spec.Mandatory
}
