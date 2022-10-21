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
	"time"

	"github.com/Masterminds/semver/v3"
	ocm "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/gardener/component-spec/bindings-go/codec"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ModuleTemplateSpec defines the desired state of ModuleTemplate.
type ModuleTemplateSpec struct {
	// Channel is the targeted channel of the ModuleTemplate. It will be used to directly assign a Template
	// to a target channel. It has to be provided at any given time.
	Channel Channel `json:"channel"`

	// Data is the default set of attributes that are used to generate the Module. It contains a default set of values
	// for a given channel, and is thus different from default values allocated during struct parsing of the Module.
	// While Data can change after the initial creation of ModuleTemplate, it is not expected to be propagated to
	// downstream modules as it is considered a set of default values. This means that an update of the data block
	// will only propagate to new Modules created form ModuleTemplate, not any existing Module.
	//
	//+kubebuilder:pruning:PreserveUnknownFields
	//+kubebuilder:validation:XEmbeddedResource
	Data unstructured.Unstructured `json:"data,omitempty"`

	// OCMDescriptor is the Raw Open Component Model Descriptor of a Module, containing all relevant information
	// to correctly initialize a module (e.g. Charts, Manifests, References to Binaries and/or configuration)
	// For more information on Component Descriptors, see
	// https://github.com/gardener/component-spec/
	//
	// It is translated inside the Lifecycle of the Cluster and will be used by downstream controllers
	// to bootstrap and manage the module. This part is also propagated for every change of the template.
	// This means for upgrades of the Descriptor, downstream controllers will also update the dependant modules
	// (e.g. by updating the controller binary linked in a chart referenced in the descriptor)
	//
	//+kubebuilder:pruning:PreserveUnknownFields
	//+structType=atomic
	OCMDescriptor runtime.RawExtension `json:"descriptor"`

	// Target describes where the Module should later on be installed if parsed correctly. It is used as installation
	// hint by downstream controllers to determine which client implementation to use for working with the Module
	Target Target `json:"target"`

	// descriptor is the internal reference holder of the OCMDescriptor once parsed.
	// it is purposefully not exposed and also excluded from parsers and only used
	// by GetDescriptor to hold a singleton reference to avoid multiple parse efforts
	// in the reconciliation loop.
	descriptor *ocm.ComponentDescriptor `json:"-"`
}

func (in *ModuleTemplateSpec) GetDescriptor() (*ocm.ComponentDescriptor, error) {
	if in.descriptor == nil && in.OCMDescriptor.Raw != nil {
		var descriptor ocm.ComponentDescriptor
		if err := codec.Decode(in.OCMDescriptor.Raw, &descriptor, codec.DisableValidation(true)); err != nil {
			return nil, err
		}
		in.descriptor = &descriptor
	}
	return in.descriptor, nil
}

func (in *ModuleTemplateSpec) ModifyDescriptor(modify func(descriptor *ocm.ComponentDescriptor) error) error {
	descriptor, err := in.GetDescriptor()
	if err != nil {
		return err
	}

	if err := modify(descriptor); err != nil {
		return err
	}

	encodedDescriptor, err := codec.Encode(descriptor)
	if err != nil {
		return err
	}

	in.OCMDescriptor = runtime.RawExtension{Raw: encodedDescriptor}
	in.descriptor = nil
	return nil
}

func ModifyDescriptorVersion(
	modify func(version *semver.Version) string,
) func(descriptor *ocm.ComponentDescriptor) error {
	return func(descriptor *ocm.ComponentDescriptor) error {
		semVersion, err := semver.NewVersion(descriptor.Version)
		if err != nil {
			return err
		}
		newVersion := modify(semVersion)
		descriptor.Version = newVersion
		for i := range descriptor.Resources {
			descriptor.Resources[i].Version = newVersion
		}
		return nil
	}
}

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

//+kubebuilder:object:root=true

// ModuleTemplateList contains a list of ModuleTemplate.
type ModuleTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ModuleTemplate `json:"items"`
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
	SchemeBuilder.Register(&ModuleTemplate{}, &ModuleTemplateList{})
}

func (in *ModuleTemplate) SetLastSync() *ModuleTemplate {
	lastSyncDate := time.Now().Format(time.RFC3339)

	if in.Annotations == nil {
		in.Annotations = make(map[string]string)
	}

	in.Annotations[LastSync] = lastSyncDate

	return in
}
