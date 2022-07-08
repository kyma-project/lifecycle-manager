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
	Name string `json:"name"`

	ControllerName string `json:"controller,omitempty"`

	Channel Channel `json:"channel,omitempty"`

	//+kubebuilder:pruning:PreserveUnknownFields
	//+kubebuilder:validation:XEmbeddedResource
	Settings unstructured.Unstructured `json:"settings,omitempty"`

	Overrides `json:"overrides,omitempty"`
}

type SyncStrategy string

const (
	SyncStrategyLocalSecret = "local-secret"
)

// Sync defines settings used to apply the kyma synchronization to other clusters.
type Sync struct {
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
	Channel Channel `json:"channel"`

	Profile Profile `json:"profile"`

	// Modules specifies the list of components to be installed
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
}

// +kubebuilder:validation:Enum=evaluation;production
type Profile string

const (
	DefaultProfile            = ProfileProduction
	ProfileEvaluation Profile = "evaluation"
	ProfileProduction Profile = "production"
)

// +kubebuilder:validation:Enum=rapid;regular;stable
type Channel string

const (
	DefaultChannel         = ChannelStable
	ChannelRapid   Channel = "rapid"
	ChannelRegular Channel = "regular"
	ChannelStable  Channel = "stable"
)

// +kubebuilder:validation:Enum=Processing;Deleting;Ready;Error
type KymaState string

// Valid Kyma States.
const (
	// KymaStateReady signifies Kyma is ready.
	KymaStateReady KymaState = "Ready"

	// KymaStateProcessing signifies Kyma is reconciling.
	KymaStateProcessing KymaState = "Processing"

	// KymaStateError signifies an error for Kyma.
	KymaStateError KymaState = "Error"

	// KymaStateDeleting signifies Kyma is being deleted.
	KymaStateDeleting KymaState = "Deleting"
)

// KymaCondition describes condition information for Kyma.
type KymaCondition struct {
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

	// Additional Information when the condition is bound to a template
	// +optional
	TemplateInfo TemplateInfo `json:"templateInfo,omitempty"`

	// Timestamp for when Kyma last transitioned from one status to another.
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
}

type TemplateInfo struct {
	Generation       int64                   `json:"generation,omitempty"`
	Channel          Channel                 `json:"channel,omitempty"`
	GroupVersionKind metav1.GroupVersionKind `json:"gvk,omitempty"`
}

type KymaConditionType string

const (
	// ConditionTypeReady represents KymaConditionType Ready.
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
