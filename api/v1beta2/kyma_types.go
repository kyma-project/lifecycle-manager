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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:storageversion
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=".status.state"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Kyma is the Schema for the kymas API.
type Kyma struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KymaSpec   `json:"spec,omitempty"`
	Status KymaStatus `json:"status,omitempty"`
}

// KymaSpec defines the desired state of Kyma.
type KymaSpec struct {
	// Channel specifies the desired Channel of the Installation, usually targeting different module versions.
	// +kubebuilder:validation:Pattern:=^[a-z]+$
	// +kubebuilder:validation:MaxLength:=32
	// +kubebuilder:validation:MinLength:=3
	Channel string `json:"channel"`

	// Modules specifies the list of modules to be installed
	Modules []Module `json:"modules,omitempty"`
}

// Module defines the components to be installed.
type Module struct {
	// Name is a unique identifier of the module.
	// It is used to resolve a ModuleTemplate for creating a set of resources on the cluster.
	//
	// Name can be one of 3 kinds:
	// - The ModuleName label value of the module-template, e.g. operator.kyma-project.io/module-name=my-module
	// - The Name or Namespace/Name of a ModuleTemplate, e.g. my-moduletemplate or kyma-system/my-moduletemplate
	// - The FQDN, e.g. kyma-project.io/module/my-module as located in .spec.descriptor.component.name
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

	// +kubebuilder:default:=CreateAndDelete
	CustomResourcePolicy `json:"customResourcePolicy,omitempty"`
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

// SyncStrategy determines how the Remote Cluster is synchronized with the Control Plane. This can influence secret
// lookup, or other behavioral patterns when interacting with the remote cluster.
type SyncStrategy string

const (
	SyncStrategyLocalSecret = "local-secret"
	SyncStrategyLocalClient = "local-client"
)

func (kyma *Kyma) GetModuleStatusMap() map[string]*ModuleStatus {
	moduleStatusMap := make(map[string]*ModuleStatus)
	for i := range kyma.Status.Modules {
		moduleStatus := &kyma.Status.Modules[i]
		moduleStatusMap[moduleStatus.Name] = moduleStatus
	}
	return moduleStatusMap
}

// KymaStatus defines the observed state of Kyma
// +kubebuilder:subresource:status
type KymaStatus struct {
	// State signifies current state of Kyma.
	// Value can be one of ("Ready", "Processing", "Error", "Deleting").
	State State `json:"state,omitempty"`

	// List of status conditions to indicate the status of a ServiceInstance.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Contains essential information about the current deployed module
	Modules []ModuleStatus `json:"modules,omitempty"`

	// Active Channel
	// +optional
	ActiveChannel string `json:"activeChannel,omitempty"`

	LastOperation `json:"lastOperation,omitempty"`
}

type LastOperation struct {
	Operation      string      `json:"operation"`
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
}

type ModuleStatus struct {
	// Name defines the name of the Module in the Spec that the status is used for.
	// It can be any kind of Reference format supported by Module.Name.
	Name string `json:"name"`

	// FQDN is the fully qualified domain name of the module.
	// In the ModuleTemplate it is located in .spec.descriptor.component.name of the ModuleTemplate
	// FQDN is used to calculate Namespace and Name of the Manifest for tracking.
	FQDN string `json:"fqdn"`

	// Manifest contains the Information of a related Manifest
	Manifest TrackingObject `json:"manifest,omitempty"`

	// It contains information about the last parsed ModuleTemplate in Context of the Installation.
	// This will update when Channel or the ModuleTemplate is changed.
	// +optional
	Template TrackingObject `json:"template"`

	// Channel tracks the active Channel of the Module. In Case it changes, the new Channel will have caused
	// a new lookup to be necessary that maybe picks a different ModuleTemplate, which is why we need to reconcile.
	Channel string `json:"channel,omitempty"`

	// Channel tracks the active Version of the Module.
	Version string `json:"version"`

	// State of the Module in the currently tracked Generation
	State State `json:"state"`
}

// TrackingObject contains metav1.TypeMeta and PartialMeta to allow a generation based object tracking.
// It purposefully does not use ObjectMeta as the generation of controller-runtime for crds would not validate
// the generation fields even when embedding ObjectMeta.
type TrackingObject struct {
	metav1.TypeMeta `json:",inline"`
	PartialMeta     `json:"metadata,omitempty"`
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

const DefaultChannel = "regular"

// +kubebuilder:validation:Enum=Processing;Deleting;Ready;Error;""
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

func AllKymaStates() []State {
	return []State{StateReady, StateProcessing, StateError, StateDeleting}
}

func PartialMetaFromObject(object metav1.Object) PartialMeta {
	return PartialMeta{
		Name:       object.GetName(),
		Namespace:  object.GetNamespace(),
		Generation: object.GetGeneration(),
	}
}

func (m PartialMeta) GetName() string {
	return m.Name
}

func (m PartialMeta) GetNamespace() string {
	return m.Namespace
}

func (m PartialMeta) GetGeneration() int64 {
	return m.Generation
}

// KymaConditionType is a programmatic identifier indicating the type for the corresponding condition.
// By combining of condition type, status and reason  it explains the current Kyma status.
// Name example:
// Type: Modules, Reason: Ready and Status: True means all modules are in ready state.
// Type: Modules, Reason: Ready and Status: False means some modules are not in ready state,
// and the actual state of individual module can be found in related ModuleStatus.
type KymaConditionType string

// KymaConditionMsg represents the current state of a condition in a human-readable format.
type KymaConditionMsg string

// KymaConditionReason should always be set to `Ready`.
type KymaConditionReason string

func (kyma *Kyma) SetActiveChannel() *Kyma {
	kyma.Status.ActiveChannel = kyma.Spec.Channel

	return kyma
}

type moduleStatusExistsPair struct {
	moduleStatus *ModuleStatus
	exists       bool
}

func (kyma *Kyma) GetNoLongerExistingModuleStatus() []*ModuleStatus {
	moduleStatusMap := make(map[string]*moduleStatusExistsPair)

	for i := range kyma.Status.Modules {
		moduleStatus := &kyma.Status.Modules[i]
		moduleStatusMap[moduleStatus.Name] = &moduleStatusExistsPair{exists: false, moduleStatus: moduleStatus}
	}

	for i := range kyma.Spec.Modules {
		module := &kyma.Spec.Modules[i]
		if _, found := moduleStatusMap[module.Name]; found {
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

func (kyma *Kyma) UpdateCondition(conditionType KymaConditionType, status metav1.ConditionStatus) {
	meta.SetStatusCondition(&kyma.Status.Conditions, metav1.Condition{
		Type:               string(conditionType),
		Status:             status,
		Reason:             string(ConditionReason),
		Message:            GenerateMessage(conditionType, status),
		ObservedGeneration: kyma.GetGeneration(),
	})
}

func (kyma *Kyma) ContainsCondition(conditionType KymaConditionType, conditionStatus ...metav1.ConditionStatus,
) bool {
	for _, existingCondition := range kyma.Status.Conditions {
		if existingCondition.Type != string(conditionType) {
			continue
		}
		if len(conditionStatus) > 0 {
			for i := range conditionStatus {
				if existingCondition.Status == conditionStatus[i] {
					return true
				}
			}
		} else {
			return true
		}
	}
	return false
}

func (kyma *Kyma) SkipReconciliation() bool {
	return kyma.GetLabels() != nil && kyma.GetLabels()[SkipReconcileLabel] == "true"
}

func (kyma *Kyma) DetermineState() State {
	status := &kyma.Status
	for _, moduleStatus := range status.Modules {
		if moduleStatus.State == StateError {
			return StateError
		}
	}

	for _, condition := range status.Conditions {
		if condition.Status != metav1.ConditionTrue {
			return StateProcessing
		}
	}

	return StateReady
}

func (kyma *Kyma) AllModulesReady() bool {
	for i := range kyma.Status.Modules {
		moduleStatus := &kyma.Status.Modules[i]
		if moduleStatus.State != StateReady {
			return false
		}
	}
	return true
}

func (kyma *Kyma) SyncEnabled() bool {
	// TODO: revisit it after 542 merged
	return false
}
