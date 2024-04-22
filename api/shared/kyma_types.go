package shared

import apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// KymaStatus defines the observed state of Kyma
// +k8s:deepcopy-gen=true
type KymaStatus struct {
	// State signifies current state of Kyma.
	// Value can be one of ("Ready", "Processing", "Error", "Deleting").
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

	LastOperation `json:"lastOperation,omitempty"`
}

// +k8s:deepcopy-gen=true
type ModuleStatus struct {
	// Name defines the name of the Module in the Spec that the status is used for.
	// It can be any kind of Reference format supported by Module.Name.
	Name string `json:"name"`

	// FQDN is the fully qualified domain name of the module.
	// In the ModuleTemplate it is located in .spec.descriptor.component.name of the ModuleTemplate
	// FQDN is used to calculate Namespace and Name of the Manifest for tracking.
	FQDN string `json:"fqdn,omitempty"`

	// Manifest contains the Information of a related Manifest
	Manifest *TrackingObject `json:"manifest,omitempty"`

	// It contains information about the last parsed ModuleTemplate in Context of the Installation.
	// This will update when Channel or the ModuleTemplate is changed.
	// +optional
	Template *TrackingObject `json:"template,omitempty"`

	// Channel tracks the active Channel of the Module. In Case it changes, the new Channel will have caused
	// a new lookup to be necessary that maybe picks a different ModuleTemplate, which is why we need to reconcile.
	Channel string `json:"channel,omitempty"`

	// Channel tracks the active Version of the Module.
	Version string `json:"version,omitempty"`

	// Message is a human-readable message indicating details about the State.
	Message string `json:"message,omitempty"`

	// State of the Module in the currently tracked Generation
	State State `json:"state"`

	// Resource contains information about the created module CR.
	Resource *TrackingObject `json:"resource,omitempty"`
}

// TrackingObject contains TypeMeta and PartialMeta to allow a generation based object tracking.
// It purposefully does not use ObjectMeta as the generation of controller-runtime for crds would not validate
// the generation fields even when embedding ObjectMeta.
// +k8s:deepcopy-gen=true
type TrackingObject struct {
	apimetav1.TypeMeta `json:",inline"`
	PartialMeta        `json:"metadata,omitempty"`
}

// PartialMeta is a subset of ObjectMeta that contains relevant information to track an Object.
// see https://github.com/kubernetes/apimachinery/blob/v0.26.1/pkg/apis/meta/v1/types.go#L111
// +k8s:deepcopy-gen=true
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

func (m PartialMeta) GetName() string {
	return m.Name
}

func (m PartialMeta) GetNamespace() string {
	return m.Namespace
}

func (m PartialMeta) GetGeneration() int64 {
	return m.Generation
}
