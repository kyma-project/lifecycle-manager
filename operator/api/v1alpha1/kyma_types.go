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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type OverrideType string

const (
	OverrideTypeHelmValues = "helm-values"
)

type (
	Overrides []Override
	Override  struct {
		Name                  string `json:"name"`
		*metav1.LabelSelector `json:"selector,omitempty"`
	}
)

type Modules []Module

// Module defines the components to be installed.
type Module struct {
	// Name is a unique identifier of the module.
	// It is used together with KymaName, ChannelLabel, ProfileLabel label to resolve a ModuleTemplate.
	Name string `json:"name"`

	// ControllerName is able to set the controller used for reconciliation of the module. It can be used
	// together with Cache Configuration on the Operator responsible for the templated Modules to split
	// workload.
	ControllerName string `json:"controller,omitempty"`

	// Channel is the desired channel of the Module. If this changes or is set, it will be used to resolve a new
	// ModuleTemplate based on the new resolved resources.
	Channel Channel `json:"channel,omitempty"`

	// Settings are a generic Representation of the entire Specification of a Module. It can be used as an alternative
	// to generic Settings written into the ModuleTemplate as they are directly passed to the resulting CR.
	// Note that this Settings argument is validated against the API Server and thus will not accept GVKs that are not
	// registered as CustomResourceDefinition. This can be used to apply settings / overrides that the operator accepts
	// as generic overrides for its CustomResource.
	//+kubebuilder:pruning:PreserveUnknownFields
	//+kubebuilder:validation:XEmbeddedResource
	Settings unstructured.Unstructured `json:"settings,omitempty"`

	// Overrides are a typed Representation of the Specification Values of a Module. It can be used to define
	// certain types of override configurations that can be used to target specific override Interfaces.
	Overrides `json:"overrides,omitempty"`
}

// SyncStrategy determines how the Remote Cluster is synchronized with the Control Plane. This can influence secret
// lookup, or other behavioral patterns when interacting with the remote cluster.
type SyncStrategy string

const (
	SyncStrategyLocalSecret = "local-secret"
)

// Sync defines settings used to apply the kyma synchronization to other clusters. This is defaulted to false
// and NOT INTENDED FOR PRODUCTIVE USE.
type Sync struct {
	// +kubebuilder:default:=false
	// Enabled set to true will look up a kubeconfig for the remote cluster based on the strategy
	// and synchronize its state there.
	Enabled bool `json:"enabled,omitempty"`

	// +kubebuilder:default:=secret
	// Strategy determines the way to lookup the remotely synced kubeconfig, by default it is fetched from a secret
	Strategy SyncStrategy `json:"strategy,omitempty"`

	// The target namespace, if empty the namespace is reflected from the control plane
	// Note that cleanup is currently not supported if you are switching the namespace, so you will
	// manually need to cleanup old synchronized Kymas
	Namespace string `json:"namespace,omitempty"`
}

// KymaSpec defines the desired state of Kyma.
type KymaSpec struct {
	// Channel specifies the desired Channel of the Installation, usually targeting different module versions.
	Channel Channel `json:"channel"`

	// Profile specifies the desired Profile of the Installation, usually targeting different resource limitations.
	Profile Profile `json:"profile"`

	// Modules specifies the list of modules to be installed
	Modules []Module `json:"modules,omitempty"`

	// Active Synchronization Settings
	// +optional
	Sync Sync `json:"sync,omitempty"`
}

func (kyma *Kyma) AreAllReadyConditionsSetForKyma() bool {
	status := &kyma.Status
	if len(status.Conditions) < 1 {
		return false
	}

	for _, existingCondition := range status.Conditions {
		if existingCondition.Type == ConditionTypeReady &&
			existingCondition.Status != ConditionStatusTrue &&
			existingCondition.Reason != KymaKind {
			return false
		}
	}

	return true
}

// KymaStatus defines the observed state of Kyma
// +kubebuilder:subresource:status
type KymaStatus struct {
	// State signifies current state of Kyma.
	// Value can be one of ("Ready", "Processing", "Error", "Deleting").
	State KymaState `json:"state,omitempty"`

	// List of status conditions to indicate the status of a ServiceInstance.
	// +optional
	Conditions []KymaCondition `json:"conditions,omitempty"`

	// Observed generation
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Active Channel
	// +optional
	ActiveChannel Channel `json:"activeChannel,omitempty"`

	ActiveOverrides map[string]*ActiveOverride `json:"activeOverrides,omitempty"`
}

type ActiveOverride struct {
	Hash    string `json:"hash,omitempty"`
	Applied bool   `json:"applied,omitempty"`
}

func (kyma *Kyma) HasOutdatedOverrides() bool {
	for _, override := range kyma.Status.ActiveOverrides {
		if !override.Applied {
			return true
		}
	}
	return false
}

func (kyma *Kyma) HasOutdatedOverride(module string) bool {
	for overwrittenModule, override := range kyma.Status.ActiveOverrides {
		if overwrittenModule == module && !override.Applied {
			return true
		}
	}
	return false
}

func (kyma *Kyma) GetModuleInfo(module string) *ModuleInfo {
	for _, existingCondition := range kyma.Status.Conditions {
		if existingCondition.Reason == module {
			return &existingCondition.ModuleInfo
		}
	}
	return nil
}

func (kyma *Kyma) RefreshOverride(module string) {
	for overwrittenModule, override := range kyma.Status.ActiveOverrides {
		if overwrittenModule == module && !override.Applied {
			override.Applied = true
			break
		}
	}
}

// +kubebuilder:validation:Enum=evaluation;production
type Profile string

const (
	DefaultProfile            = ProfileProduction
	ProfileEvaluation Profile = "evaluation"
	ProfileProduction Profile = "production"
)

// Channel is the release channel in which a Kyma Instance is running. It is used for running Kyma Installations
// in a control plane against different stability levels of our module system. When switching Channel, all modules
// will be recalculated based on new templates. If you did not configure a ModuleTemplate for the new channel, the Kyma
// will abort the installation.
// +kubebuilder:validation:Enum=rapid;regular;stable
type Channel string

const (
	DefaultChannel = ChannelStable
	// ChannelRapid is meant as a fast track channel that will always be equal or close to the main codeline.
	ChannelRapid Channel = "rapid"
	// ChannelRegular is meant as the next best Ugrade path and a median between "bleeding edge" and stability.
	ChannelRegular Channel = "regular"
	// ChannelStable is meant as a reference point and should be used for productive installations.
	ChannelStable Channel = "stable"
)

// +kubebuilder:validation:Enum=Processing;Deleting;Ready;Error
type KymaState string

// Valid Kyma States.
const (
	// KymaStateReady signifies Kyma is ready and has been installed successfully.
	KymaStateReady KymaState = "Ready"

	// KymaStateProcessing signifies Kyma is reconciling and is in the process of installation. Processing can also
	// signal that the Installation previously encountered an error and is now recovering.
	KymaStateProcessing KymaState = "Processing"

	// KymaStateError signifies an error for Kyma. This signifies that the Installation process encountered an error.
	// Contrary to Processing, it can be expected that this state should change on the next retry.
	KymaStateError KymaState = "Error"

	// KymaStateDeleting signifies Kyma is being deleted. This is the state that is used when a deletionTimestamp
	// was detected and Finalizers are picked up.
	KymaStateDeleting KymaState = "Deleting"
)

// KymaCondition describes condition information for Kyma.
type KymaCondition struct {
	// Type is used to reflect what type of condition we are dealing with. Most commonly ConditionTypeReady it is used
	// as extension marker in the future
	Type KymaConditionType `json:"type"`

	// Status of the Kyma Condition.
	// Value can be one of ("True", "False", "Unknown").
	Status KymaConditionStatus `json:"status"`

	// Human-readable message indicating details about the last status transition.
	// +optional
	Message string `json:"message,omitempty"`

	// Machine-readable text indicating the reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Additional Information when the condition is bound to a ModuleTemplate. It contains information about the last
	// parsing that occurred and will track the state of the parser ModuleTemplate in Context of the Installation.
	// This will update when Channel, Profile or the ModuleTemplate used in the Condition is changed.
	// +optional
	TemplateInfo TemplateInfo `json:"templateInfo,omitempty"`

	// ModuleInfo provide the latest deployed Module CR information.
	ModuleInfo ModuleInfo `json:"moduleInfo,omitempty"`

	// Timestamp for when Kyma last transitioned from one status to another.
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
}

type ModuleInfo struct {
	// Generation tracks the active Generation of the Deployed Module CR.
	Generation int64 `json:"generation,omitempty"`
}

type TemplateInfo struct {
	// Generation tracks the active Generation of the ModuleTemplate. In Case it changes, the new Generation will differ
	// from the one tracked in TemplateInfo and thus trigger a new reconciliation with a newly parser ModuleTemplate
	Generation int64 `json:"generation,omitempty"`

	// Channel tracks the active Channel of the ModuleTemplate. In Case it changes, the new Channel will have caused
	// a new lookup to be necessary that maybe picks a different ModuleTemplate, which is why we need to reconcile.
	Channel Channel `json:"channel,omitempty"`

	// GroupVersionKind is used to track the Kind that was created from the ModuleTemplate. This is dynamic to not bind
	// ourselves to any kind of Kind in the code and allows us to work generic on deletion / cleanup of
	// related resources to a Kyma Installation.
	GroupVersionKind metav1.GroupVersionKind `json:"gvk,omitempty"`
}

type KymaConditionType string

const (
	// ConditionTypeReady represents KymaConditionType Ready, meaning as soon as its true we will reconcile Kyma
	// into KymaStateReady.
	ConditionTypeReady KymaConditionType = "Ready"
)

type KymaConditionStatus string

// Valid KymaCondition Status.
const (
	// ConditionStatusTrue signifies KymaConditionStatus true.
	ConditionStatusTrue KymaConditionStatus = "True"

	// ConditionStatusFalse signifies KymaConditionStatus false.
	ConditionStatusFalse KymaConditionStatus = "False"

	// ConditionStatusUnknown signifies KymaConditionStatus unknown.
	ConditionStatusUnknown KymaConditionStatus = "Unknown"
)

//+genclient
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=".status.state"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Kyma is the Schema for the kymas API.
type Kyma struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KymaSpec   `json:"spec,omitempty"`
	Status KymaStatus `json:"status,omitempty"`
}

func (kyma *Kyma) SetObservedGeneration() *Kyma {
	kyma.Status.ObservedGeneration = kyma.Generation

	return kyma
}

func (kyma *Kyma) SetActiveChannel() *Kyma {
	kyma.Status.ActiveChannel = kyma.Spec.Channel

	return kyma
}

//+kubebuilder:object:root=true

// KymaList contains a list of Kyma.
type KymaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Kyma `json:"items"`
}

//nolint:gochecknoinits
func init() {
	SchemeBuilder.Register(&Kyma{}, &KymaList{})
}
