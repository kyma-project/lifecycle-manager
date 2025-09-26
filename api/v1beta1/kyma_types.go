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

	Spec   KymaSpec   `json:"spec,omitempty"`
	Status KymaStatus `json:"status,omitempty"`
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
	Strategy SyncStrategy `json:"strategy,omitempty"`

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

// SyncStrategy determines how the Remote Cluster is synchronized with the Control Plane. This can influence secret
// lookup, or other behavioral patterns when interacting with the remote cluster.
type SyncStrategy string

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

// KymaStatus defines the observed state of Kyma.
type KymaStatus struct {
	LastOperation `json:"lastOperation,omitempty"`

	// State signifies current state of Kyma.
	// Value can be one of ("Ready", "Processing", "Warning", "Error", "Deleting").
	// Note: The requeue interval in Error State is subject to rate limiting.
	State State `json:"state,omitempty"`

	// List of status conditions to indicate the status of a ServiceInstance.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []apimetav1.Condition `json:"conditions,omitempty"`

	// Contains essential information about the current deployed module
	Modules []ModuleStatus `json:"modules,omitempty"`

	// Active Channel
	// +optional
	ActiveChannel string `json:"activeChannel,omitempty"`
}

type ModuleStatus struct {
	// Name defines the name of the Module in the Spec that the status is used for.
	// It can be any kind of Reference format supported by Module.Name.
	Name string `json:"name"`

	// FQDN is the fully qualified domain name of the module.
	// In the ModuleTemplate it is located in .spec.descriptor.component.name of the ModuleTemplate
	// FQDN is used to calculate Namespace and Name of the Manifest for tracking.
	FQDN string `json:"fqdn,omitempty"`

	// Channel tracks the active Channel of the Module. In Case it changes, the new Channel will have caused
	// a new lookup to be necessary that maybe picks a different ModuleTemplate, which is why we need to reconcile.
	Channel string `json:"channel,omitempty"`

	// Channel tracks the active Version of the Module.
	Version string `json:"version,omitempty"`

	// Message is a human-readable message indicating details about the State.
	Message string `json:"message,omitempty"`

	// State of the Module in the currently tracked Generation
	State State `json:"state"`

	// Manifest contains the Information of a related Manifest
	Manifest *TrackingObject `json:"manifest,omitempty"`

	// Resource contains information about the created module CR.
	Resource *TrackingObject `json:"resource,omitempty"`

	// It contains information about the last parsed ModuleTemplate in Context of the Installation.
	// This will update when Channel or the ModuleTemplate is changed.
	// +optional
	Template *TrackingObject `json:"template,omitempty"`
}

// TrackingObject contains TypeMeta and PartialMeta to allow a generation based object tracking.
// It purposefully does not use ObjectMeta as the generation of controller-runtime for crds would not validate
// the generation fields even when embedding ObjectMeta.
type TrackingObject struct {
	apimetav1.TypeMeta `json:",inline"`
	PartialMeta        `json:"metadata,omitempty"`
}

// PartialMeta is a subset of ObjectMeta that contains relevant information to track an Object.
// see https://github.com/kubernetes/apimachinery/blob/v0.26.1/pkg/apis/meta/v1/types.go#L111
type PartialMeta struct {
	// Name must be unique within a namespace. Is required when creating resources, although
	// some resources may allow a client to request the generation of an appropriate name
	// automatically. Name is primarily intended for creation idempotence and configuration
	// definition.
	// Cannot be updated.
	// More info: http://kubernetes.io/docs/user-guide/identifiers#names
	// +optional
	Name string `json:"name"`
	// Namespace defines the space within which each name must be unique. An empty namespace is
	// equivalent to the "default" namespace, but "default" is the canonical representation.
	// Not all objects are required to be scoped to a namespace - the value of this field for
	// those objects will be empty.
	//
	// Must be a DNS_LABEL.
	// Cannot be updated.
	// More info: http://kubernetes.io/docs/user-guide/namespaces
	// +optional
	Namespace string `json:"namespace"`
	// A sequence number representing a specific generation of the desired state.
	// Populated by the system. Read-only.
	// +optional
	Generation int64 `json:"generation,omitempty"`
}

// +kubebuilder:object:root=true

// KymaList contains a list of Kyma.
type KymaList struct {
	apimetav1.TypeMeta `json:",inline"`
	apimetav1.ListMeta `json:"metadata,omitempty"`

	Items []Kyma `json:"items"`
}

// Module defines the components to be installed.
type Module struct {
	// +kubebuilder:default:=CreateAndDelete
	CustomResourcePolicy `json:"customResourcePolicy,omitempty"`

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
