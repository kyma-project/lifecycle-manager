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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

//+genclient
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=".status.state"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:deprecatedversion:warning="kyma-project.io/v1beta1 Kyma is deprecated. Use v1beta2 instead."
//+kubebuilder:storageversion

// Kyma is the Schema for the kymas API.
type Kyma struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

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

	// Modules specifies the list of modules to be installed
	Modules []v1beta2.Module `json:"modules,omitempty"`

	// Active Synchronization Settings
	// +optional
	Sync Sync `json:"sync,omitempty"`
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
