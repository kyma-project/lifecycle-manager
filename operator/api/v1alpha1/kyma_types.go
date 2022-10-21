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
	"errors"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type OverrideType string

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
	//
	// WARNING: Module-Names are restricted in length based on naming generation strategy!
	// By default, this means that the length of Name and .metadata.name of Kyma combined must be <= 252 Characters
	// This is because the naming strategy aggregates Kyma and Module into a format of "kyma-name-module-name"
	// For more info on the 253 total character limit, see
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names
	Name string `json:"name"`

	// ControllerName is able to set the controller used for reconciliation of the module. It can be used
	// together with Cache Configuration on the Operator responsible for the templated Modules to split
	// workload.
	ControllerName string `json:"controller,omitempty"`

	// Channel is the desired channel of the Module. If this changes or is set, it will be used to resolve a new
	// ModuleTemplate based on the new resolved resources.
	Channel Channel `json:"channel,omitempty"`
}

// SyncStrategy determines how the Remote Cluster is synchronized with the Control Plane. This can influence secret
// lookup, or other behavioral patterns when interacting with the remote cluster.
type SyncStrategy string

const (
	SyncStrategyLocalSecret = "local-secret"
	SyncStrategyLocalClient = "local-client"
)

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

// KymaSpec defines the desired state of Kyma.
type KymaSpec struct {
	// Channel specifies the desired Channel of the Installation, usually targeting different module versions.
	Channel Channel `json:"channel"`

	// Modules specifies the list of modules to be installed
	Modules []Module `json:"modules,omitempty"`

	// Active Synchronization Settings
	// +optional
	Sync Sync `json:"sync,omitempty"`
}

func (kyma *Kyma) AreAllConditionsReadyForKyma() bool {
	status := &kyma.Status
	if len(status.Conditions) < 1 {
		return false
	}

	for _, existingCondition := range status.Conditions {
		if existingCondition.Type == string(ConditionTypeReady) &&
			existingCondition.Status != metav1.ConditionTrue {
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
	State State `json:"state,omitempty"`

	// List of status conditions to indicate the status of a ServiceInstance.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Contains essential information about the current deployed module
	ModuleStatus []ModuleStatus `json:"moduleStatus,omitempty"`

	// Active Channel
	// +optional
	ActiveChannel Channel `json:"activeChannel,omitempty"`
}

// Channel is the release channel in which a Kyma Instance is running. It is used for running Kyma Installations
// in a control plane against different stability levels of our module system. When switching Channel, all modules
// will be recalculated based on new templates. If you did not configure a ModuleTemplate for the new channel, the Kyma
// will abort the installation.
// +kubebuilder:validation:Enum=rapid;fast;regular;stable
type Channel string

//goland:noinspection ALL
const (
	DefaultChannel = ChannelStable
	// ChannelFast is meant as a fast track channel that will always be equal or close to the main codeline.
	// Alias for ChannelRapid.
	ChannelFast Channel = "fast"
	// ChannelRapid is meant as a fast track channel that will always be equal or close to the main codeline.
	// Alias for ChannelFast.
	ChannelRapid Channel = "rapid"
	// ChannelRegular is meant as the next best Ugrade path and a median between "bleeding edge" and stability.
	ChannelRegular Channel = "regular"
	// ChannelStable is meant as a reference point and should be used for productive installations.
	ChannelStable Channel = "stable"
)

// +kubebuilder:validation:Enum=Processing;Deleting;Ready;Error
type State string

// Valid States.
const (
	// StateReady signifies specified resource is ready and has been installed successfully.
	StateReady State = "Ready"

	// StateProcessing signifies specified resource is reconciling and is in the process of installation.
	// Processing can also signal that the Installation previously encountered an error and is now recovering.
	StateProcessing State = "Processing"

	// StateError signifies an error for specified resource.
	// This signifies that the Installation process encountered an error.
	// Contrary to Processing, it can be expected that this state should change on the next retry.
	StateError State = "Error"

	// StateDeleting signifies specified resource is being deleted. This is the state that is used when a deletionTimestamp
	// was detected and Finalizers are picked up.
	StateDeleting State = "Deleting"
)

type ModuleStatus struct {
	// Name is the current deployed module name
	Name string `json:"name"`

	// ModuleName is the unique identifier of the module.
	ModuleName string `json:"moduleName"`

	// It contains information about the last parsed ModuleTemplate in Context of the Installation.
	// This will update when Channel or the ModuleTemplate is changed.
	// +optional
	TemplateInfo TemplateInfo `json:"templateInfo"`

	// Namespace is the current deployed module namespace
	Namespace string `json:"namespace"`

	// status of the condition, one of True, False, Unknown.
	State State `json:"state"`
}

type TemplateInfo struct {
	// Name is the current name of the template resource referenced
	Name string `json:"name"`

	// Namespace is the namespace of the template
	Namespace string `json:"namespace"`

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

	Version string `json:"version"`
}

type KymaConditionType string

const (
	// ConditionTypeReady represents KymaConditionType Ready, meaning as soon as its true we will reconcile Kyma
	// into KymaStateReady.
	ConditionTypeReady KymaConditionType = "Ready"
)

// KymaConditionReason is a programmatic identifier indicating the reason for the condition's last transition.
// By combining of condition status, it explains the current Kyma status for all modules.
// For example:
// Reason: ModulesIsReady and Status: True means all modules are in ready state.
// Reason: ModulesIsReady and Status: False means some modules are not in ready state,
// and the actual state of individual module can be found in related ModuleStatus.
type KymaConditionReason string

// Extend this list by actual needs.
const (
	ConditionReasonModulesAreReady      KymaConditionReason = "ModulesAreReady"
	ConditionReasonModuleCatalogIsReady KymaConditionReason = "ModuleCatalogIsReady"
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

func (kyma *Kyma) SetActiveChannel() *Kyma {
	kyma.Status.ActiveChannel = kyma.Spec.Channel

	return kyma
}

func (kyma *Kyma) SetLastSync() *Kyma {
	// this is an additional update on the runtime and might not be worth it
	lastSyncDate := time.Now().Format(time.RFC3339)
	if kyma.Annotations == nil {
		kyma.Annotations = make(map[string]string)
	}
	kyma.Annotations[LastSync] = lastSyncDate

	return kyma
}

type moduleStatusExistsPair struct {
	moduleStatus *ModuleStatus
	exists       bool
}

func (kyma *Kyma) GetNoLongerExistingModuleStatus() []*ModuleStatus {
	moduleStatusMap := make(map[string]*moduleStatusExistsPair)

	for i := range kyma.Status.ModuleStatus {
		moduleStatus := &kyma.Status.ModuleStatus[i]
		moduleStatusMap[moduleStatus.ModuleName] = &moduleStatusExistsPair{exists: false, moduleStatus: moduleStatus}
	}

	for i := range kyma.Spec.Modules {
		module := &kyma.Spec.Modules[i]
		if _, exists := moduleStatusMap[module.Name]; exists {
			moduleStatusMap[module.Name].exists = true
		}
	}

	notExistsModules := make([]*ModuleStatus, 0)
	for _, item := range moduleStatusMap {
		if !item.exists {
			notExistsModules = append(notExistsModules, item.moduleStatus)
		}
	}
	return notExistsModules
}

func (kyma *Kyma) GetModuleStatusMap() map[string]*ModuleStatus {
	moduleStatusMap := make(map[string]*ModuleStatus)
	for i := range kyma.Status.ModuleStatus {
		moduleStatus := &kyma.Status.ModuleStatus[i]
		moduleStatusMap[moduleStatus.ModuleName] = moduleStatus
	}
	return moduleStatusMap
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

func (kyma *Kyma) UpdateCondition(reason KymaConditionReason, status metav1.ConditionStatus) {
	newCondition := NewConditionBuilder().
		SetReason(reason).
		SetStatus(status).
		SetObservedGeneration(kyma.GetGeneration()).
		Build()

	isNewReason := true

	for i := range kyma.Status.Conditions {
		condition := &kyma.Status.Conditions[i]
		if condition.Reason == string(reason) {
			isNewReason = false
			if condition.Status != newCondition.Status || condition.Type != newCondition.Type {
				*condition = newCondition
			}
		}
	}

	if isNewReason {
		kyma.Status.Conditions = append(kyma.Status.Conditions, newCondition)
	}
}

func (kyma *Kyma) ContainsCondition(conditionType KymaConditionType,
	reason KymaConditionReason, conditionStatus ...metav1.ConditionStatus,
) bool {
	for _, condition := range kyma.Status.Conditions {
		reasonTypeMatch := condition.Type == string(conditionType) && condition.Reason == string(reason)
		if len(conditionStatus) > 0 {
			for i := range conditionStatus {
				if reasonTypeMatch && condition.Status == conditionStatus[i] {
					return true
				}
			}
		} else if reasonTypeMatch {
			return true
		}
	}
	return false
}

var ErrTemplateNotFound = errors.New("template not found")

func (kyma *Kyma) GetTemplateInfoByModuleName(
	moduleName string,
) (*TemplateInfo, error) {
	for i := range kyma.Status.ModuleStatus {
		moduleStatus := &kyma.Status.ModuleStatus[i]
		if moduleStatus.ModuleName == moduleName {
			return &moduleStatus.TemplateInfo, nil
		}
	}
	// should not happen
	return nil, ErrTemplateNotFound
}

func IsValidState(state string) bool {
	castedState := State(state)
	return castedState == StateReady ||
		castedState == StateProcessing ||
		castedState == StateDeleting ||
		castedState == StateError
}

// SyncConditionsWithModuleStates iterates all moduleStatus, based on all module state,
// it updates the condition.status with Reason ConditionReasonModulesAreReady accordingly.
func (kyma *Kyma) SyncConditionsWithModuleStates() {
	conditionReason := ConditionReasonModulesAreReady
	conditionStatus := metav1.ConditionTrue
	for i := range kyma.Status.ModuleStatus {
		moduleStatus := &kyma.Status.ModuleStatus[i]
		if moduleStatus.State != StateReady {
			conditionStatus = metav1.ConditionFalse
			break
		}
	}
	kyma.UpdateCondition(conditionReason, conditionStatus)
}
