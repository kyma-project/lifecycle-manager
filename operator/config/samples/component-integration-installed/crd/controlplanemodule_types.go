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

//+groupName=component.kyma-project.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ControlPlaneModuleSpec defines the desired state of ControlPlaneModule.
type ControlPlaneModuleSpec struct {
	// Config specifies OCI image configuration
	// +optional
	Config ImageSpec `json:"config"`
}

// ImageSpec defines installation.
type ImageSpec struct {
	// Repo defines the Image repo
	Repo string `json:"repo,omitempty"`

	// Name defines the Image name
	Name string `json:"name,omitempty"`

	// Ref is either a sha value, tag or version
	Ref string `json:"ref,omitempty"`

	// Type defines the chart as "oci-ref"
	// +kubebuilder:validation:Enum=helm-chart;oci-ref
	Type RefTypeMetadata `json:"type"`
}

type RefTypeMetadata string

const (
	HelmChartType RefTypeMetadata = "helm-chart"
	OciRefType    RefTypeMetadata = "oci-ref"
)

//+genclient
//+kubebuilder:object:root=true
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// ControlPlaneModule is the Schema for the moduletemplates API.
type ControlPlaneModule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ControlPlaneModuleSpec   `json:"spec,omitempty"`
	Status ControlPlaneModuleStatus `json:"status,omitempty"`
}

// +kubebuilder:validation:Enum=Processing;Deleting;Ready;Error
type ControlPlaneModuleState string

const (
	// ControlPlaneModuleStateReady signifies Manifest is ready.
	ControlPlaneModuleStateReady ControlPlaneModuleState = "Ready"

	// ControlPlaneModuleStateProcessing signifies Manifest is reconciling.
	ControlPlaneModuleStateProcessing ControlPlaneModuleState = "Processing"

	// ControlPlaneModuleStateError signifies an error for Manifest.
	ControlPlaneModuleStateError ControlPlaneModuleState = "Error"

	// ControlPlaneModuleStateDeleting signifies Manifest is being deleted.
	ControlPlaneModuleStateDeleting ControlPlaneModuleState = "Deleting"
)

// ManifestStatus defines the observed state of Manifest.
type ControlPlaneModuleStatus struct {
	State ControlPlaneModuleState `json:"state,omitempty"`
}

//+kubebuilder:object:root=true

// ControlPlaneModuleList contains a list of ControlPlaneModule.
type ControlPlaneModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ControlPlaneModule `json:"items"`
}
