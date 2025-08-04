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

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=".status.state"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:deprecatedversion:warning="kyma-project.io/v1beta1 Kyma is deprecated. Use v1beta2 instead."
// +kubebuilder:unservedversion

// Kyma is the Schema for the kymas API.
type Kyma struct {
	apimetav1.TypeMeta   `json:",inline"`
	apimetav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KymaSpec           `json:"spec,omitempty"`
	Status v1beta2.KymaStatus `json:"status,omitempty"`
}

// Sync defines settings used to apply the kyma synchronization to other clusters. This is defaulted to false
// and NOT INTENDED FOR PRODUCTIVE USE.
type Sync struct {
	// +kubebuilder:default:=false
	// Enabled set to true will look up a kubeconfig for the remote cluster based on the strategy
	// and synchronize its state there.
	Enabled bool `json:"enabled,omitempty"`

	// +kubebuilder:default:=secret
	// Strategy determines the way to look up the remotely synced kubeconfig, by default it is fetched from a secret
	Strategy v1beta2.SyncStrategy `json:"strategy,omitempty"`

	// The target namespace, if empty the namespace is reflected from the control plane
	// Note that cleanup is currently not supported if you are switching the namespace, so you will
	// manually need to clean up old synchronized Kymas
	Namespace string `json:"namespace,omitempty"`

	// +kubebuilder:default:=true
	// NoModuleCopy set to true will cause the remote Kyma to be initialized without copying over the
	// module spec of the control plane into the SKR
	NoModuleCopy bool `json:"noModuleCopy,omitempty"`

	// +kubebuilder:default:=true
	// ModuleCatalog set to true will cause a copy of all ModuleTemplate in the cluster
	// to be synchronized for discovery purposes
	ModuleCatalog bool `json:"moduleCatalog,omitempty"`
}

// KymaSpec defines the desired state of Kyma.
type KymaSpec struct {
	// Channel specifies the desired Channel of the Installation, usually targeting different module versions.
	// +kubebuilder:validation:Pattern:=^[a-z]+$
	// +kubebuilder:validation:MaxLength:=32
	// +kubebuilder:validation:MinLength:=3
	Channel string `json:"channel"`

	// SkipMaintenanceWindows indicates whether module upgrades that require downtime
	// should bypass the defined Maintenance Windows and be applied immediately.
	SkipMaintenanceWindows bool `json:"skipMaintenanceWindows,omitempty"`

	// Modules specifies the list of modules to be installed
	Modules []Module `json:"modules,omitempty"`

	// Active Synchronization Settings
	// +optional
	Sync Sync `json:"sync,omitempty"`
}

// +kubebuilder:object:root=true

// KymaList contains a list of Kyma.
type KymaList struct {
	apimetav1.TypeMeta `json:",inline"`
	apimetav1.ListMeta `json:"metadata,omitempty"`
	Items              []Kyma `json:"items"`
}

// Module defines the components to be installed.
type Module struct {
	// Name is a unique identifier of the module.
	// It is used to resolve a ModuleTemplate for creating a set of resources on the cluster.
	//
	// Name can only be the ModuleName label value of the module-template, e.g. operator.kyma-project.io/module-name=my-module
	Name string `json:"name"`

	// ControllerName is able to set the controller used for reconciliation of the module. It can be used
	// together with Cache Configuration on the Operator responsible for the templated Modules to split
	// workload.
	ControllerName string `json:"controller,omitempty"`

	// Channel is the desired channel of the Module. If this changes or is set, it will be used to resolve a new
	// ModuleTemplate based on the new resolved resources.
	// +kubebuilder:validation:Pattern:=^[a-z]+$
	// +kubebuilder:validation:MaxLength:=32
	// +kubebuilder:validation:MinLength:=3
	Channel string `json:"channel,omitempty"`

	// Version is the desired version of the Module. If this changes or is set, it will be used to resolve a new
	// ModuleTemplate based on this specific version.
	// The Version and Channel are mutually exclusive options.
	// The regular expression come from here: https://semver.org/#is-there-a-suggested-regular-expression-regex-to-check-a-semver-string
	// json:"-" to disable installation of specific versions until decided to roll this out
	// see https://github.com/kyma-project/lifecycle-manager/issues/1847
	Version string `json:"-"`

	// RemoteModuleTemplateRef is deprecated and will no longer have any functionality.
	// It will be removed in the upcoming API version.
	RemoteModuleTemplateRef string `json:"remoteModuleTemplateRef,omitempty"`

	// +kubebuilder:default:=CreateAndDelete
	CustomResourcePolicy `json:"customResourcePolicy,omitempty"`

	// Managed is determining whether the module is managed or not. If the module is unmanaged, the user is responsible
	// for the lifecycle of the module.
	// +kubebuilder:default:=true
	Managed bool `json:"managed"`
}

// CustomResourcePolicy determines how a ModuleTemplate should be parsed. When CustomResourcePolicy is set to
// CustomResourcePolicyCreateAndDelete, the Manifest will receive instructions to create it on installation with
// the default values provided in ModuleTemplate, and to remove it when the module or Kyma is deleted.
// +kubebuilder:validation:Enum=CreateAndDelete;Ignore
type CustomResourcePolicy string

const (
	// CustomResourcePolicyCreateAndDelete causes the Manifest to contain the default data provided in ModuleTemplate.
	// While Updates from the Data are never propagated, the resource is deleted on module removal.
	CustomResourcePolicyCreateAndDelete = "CreateAndDelete"
	// CustomResourcePolicyIgnore does not pass the Data from ModuleTemplate.
	// This ensures the user of the module is able to initialize the Module without any default configuration.
	// This is useful if another controller should manage module configuration as data and not be auto-initialized.
	// It can also be used to initialize controllers without interacting with them.
	CustomResourcePolicyIgnore = "Ignore"
)

//nolint:gochecknoinits // registers Kyma CRD on startup
func init() {
	SchemeBuilder.Register(&Kyma{}, &KymaList{})
}
