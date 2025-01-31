package v1beta2

import (
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModuleReleaseMeta is the representation of the channel-version pairs for modules. Each item represents
// a module version along with its assigned channel.
//
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:singular=modulereleasemeta,path=modulereleasemetas,shortName=mrm
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:storageversion

type ModuleReleaseMeta struct {
	apimetav1.TypeMeta   `json:",inline"`
	apimetav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ModuleReleaseMetaSpec `json:"spec,omitempty"`
}

// ModuleReleaseMetaSpec defines the channel-version assignments for a module.
type ModuleReleaseMetaSpec struct {
	// ModuleName is the name of the Module.
	// +kubebuilder:validation:Pattern:=`^([a-z]{3,}(-[a-z]{3,})*)?$`
	// +kubebuilder:validation:MaxLength:=64
	ModuleName string `json:"moduleName"`

	// Channels is the list of module channels with their corresponding versions.
	// +listType=map
	// +listMapKey=channel
	Channels []ChannelVersionAssignment `json:"channels"`

	// Beta indicates if the module is in beta state. Beta modules are only available for beta Kymas.
	// +optional
	// +kubebuilder:default:=false
	Beta bool `json:"beta"`

	// Internal indicates if the module is internal. Internal modules are only available for internal Kymas.
	// +optional
	// +kubebuilder:default:=false
	Internal bool `json:"internal"`
}

// +kubebuilder:object:root=true

// ModuleReleaseMetaList contains a list of ModuleReleaseMeta.
type ModuleReleaseMetaList struct {
	apimetav1.TypeMeta `json:",inline"`
	apimetav1.ListMeta `json:"metadata,omitempty"`
	Items              []ModuleReleaseMeta `json:"items"`
}

type ChannelVersionAssignment struct {
	// Channel is the module channel.
	// +kubebuilder:validation:Pattern:=^[a-z]+$
	// +kubebuilder:validation:MaxLength:=32
	// +kubebuilder:validation:MinLength:=3
	Channel string `json:"channel"`

	// Version is the module version of the corresponding module channel.
	// +kubebuilder:validation:Pattern:=`^((0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[a-zA-Z-][0-9a-zA-Z-]*)?)?$`
	// +kubebuilder:validation:MaxLength:=32
	Version string `json:"version"`
}

//nolint:gochecknoinits // registers ModuleReleaseMeta CRD on startup
func init() {
	SchemeBuilder.Register(&ModuleReleaseMeta{}, &ModuleReleaseMetaList{})
}

func (m ModuleReleaseMeta) IsBeta() bool {
	return m.Spec.Beta
}

func (m ModuleReleaseMeta) IsInternal() bool {
	return m.Spec.Internal
}
