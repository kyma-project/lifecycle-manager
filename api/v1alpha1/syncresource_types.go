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
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SyncResourceSpec defines the desired state of SyncResource
type SyncResourceSpec struct {
	// Kyma specifies related kyma name
	Kyma string `json:"kyma"`

	// SyncedItems specifies a list of resources to be synced to remote cluster, which is bound to a specific strategy defined.
	SyncedItems []SyncedItem `json:"items"`
}

type SyncStrategy string

const (
	CreateAndSync   SyncStrategy = "CreateAndSync"
	CreateAndIgnore SyncStrategy = "CreateAndIgnore"
	Delete          SyncStrategy = "Delete"
)

type SyncedItem struct {
	// +kubebuilder:validation:Enum=CreateAndSync;Delete;CreateAndIgnore
	Strategy SyncStrategy `json:"strategy"`

	// Name defines the name of the resource, it is not necessarily the same name as the actual resource, but mainly for indexing purposes
	Name string `json:"name"`

	// Resource contains the resource manifest, to be noticed, lifecycle manager only takes responsibility for syncing spec section of the resource, but not subresource, e.g: status, if the sync of subresource is required, use `SubResource` instead. A valid resource should have apiVersion, kind, metadata and spec fields.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:XEmbeddedResource
	Resource *unstructured.Unstructured `json:"resource,omitempty"`

	// SubResource contains the resource manifest, to be noticed, lifecycle manager only takes responsibility for syncing subresource section of the resource, but not spec section, if the sync of spec is required, use Resource instead. A valid subresource should have apiVersion, kind, metadata and status fields.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:XEmbeddedResource
	SubResource *unstructured.Unstructured `json:"subresource,omitempty"`
}

// SyncResourceStatus defines the observed state of SyncResource
type SyncResourceStatus struct {
	// List of status conditions to indicate the status of a synced resource.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []apimetav1.Condition `json:"conditions,omitempty"`

	shared.LastOperation `json:"lastOperation,omitempty"`

	// State signifies current state of all SyncedItem in this CR.
	State shared.State `json:"state,omitempty"`

	SyncedItemStatus []SyncedItemStatus `json:"items"`
}

type SyncedItemStatus struct {
	// Message is a human-readable message indicating details about this SyncedItem.
	Message string `json:"message"`

	// Name is the name of SyncedItem
	Name string `json:"name"`

	// State signifies current state of this SyncedItem.
	State shared.State `json:"state,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SyncResource is the Schema for the syncresources API
type SyncResource struct {
	apimetav1.TypeMeta   `json:",inline"`
	apimetav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SyncResourceSpec   `json:"spec,omitempty"`
	Status SyncResourceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SyncResourceList contains a list of SyncResource
type SyncResourceList struct {
	apimetav1.TypeMeta `json:",inline"`
	apimetav1.ListMeta `json:"metadata,omitempty"`
	Items              []SyncResource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SyncResource{}, &SyncResourceList{})
}
