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
	"fmt"
	"strings"

	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ModuleTemplate is a representation of a Template used for creating Module Instances within the Module Lifecycle.
// It is generally loosely defined within the Kubernetes Specification, however it has a strict enforcement of
// OCM guidelines as it serves an active role in maintaining a list of available Modules within a cluster.
//
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type ModuleTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

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

func (d *Descriptor) DeepCopyObject() runtime.Object {
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

	// Data is the default set of attributes that are used to generate the Module. It contains a default set of values
	// for a given channel, and is thus different from default values allocated during struct parsing of the Module.
	// While Data can change after the initial creation of ModuleTemplate, it is not expected to be propagated to
	// downstream modules as it is considered a set of default values. This means that an update of the data block
	// will only propagate to new Modules created form ModuleTemplate, not any existing Module.
	//
	//+kubebuilder:pruning:PreserveUnknownFields
	//+kubebuilder:validation:XEmbeddedResource
	Data unstructured.Unstructured `json:"data,omitempty"`

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
	//+kubebuilder:pruning:PreserveUnknownFields
	Descriptor runtime.RawExtension `json:"descriptor"`
}

func (spec *ModuleTemplateSpec) GetDescriptor(opts ...compdesc.DecodeOption) (*Descriptor, error) {
	if spec.Descriptor.Object != nil {
		return spec.Descriptor.Object.(*Descriptor), nil
	}
	desc, err := compdesc.Decode(
		spec.Descriptor.Raw, append([]compdesc.DecodeOption{compdesc.DisableValidation(true)}, opts...)...,
	)
	if err != nil {
		return nil, err
	}
	spec.Descriptor.Object = &Descriptor{ComponentDescriptor: desc}
	return spec.Descriptor.Object.(*Descriptor), err
}

//+kubebuilder:object:root=true

// ModuleTemplateList contains a list of ModuleTemplate.
type ModuleTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ModuleTemplate `json:"items"`
}

//nolint:gochecknoinits
func init() {
	SchemeBuilder.Register(&ModuleTemplate{}, &ModuleTemplateList{}, &Descriptor{})
}

func (m *ModuleTemplate) GetComponentDescriptorCacheKey() (string, error) {
	descriptor, err := m.Spec.GetDescriptor()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%s:%s", m.Spec.Channel, descriptor.GetName(), descriptor.GetVersion()), nil
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

	return true
}

func (m *ModuleTemplate) syncDisabled() bool {
	if isSync, found := m.Labels[SyncLabel]; found {
		return strings.ToLower(isSync) == DisableLabelValue
	}
	return false
}

func (m *ModuleTemplate) IsInternal() bool {
	if isInternal, found := m.Labels[InternalLabel]; found {
		return strings.ToLower(isInternal) == EnableLabelValue
	}
	return false
}

func (m *ModuleTemplate) IsBeta() bool {
	if isBeta, found := m.Labels[BetaLabel]; found {
		return strings.ToLower(isBeta) == EnableLabelValue
	}
	return false
}
