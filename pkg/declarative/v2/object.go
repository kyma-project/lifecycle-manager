package v2

import (
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:generate mockgen -source object.go -destination mock/object.go Object
type Object interface {
	client.Object
	ComponentName() string
	GetStatus() Status
	SetStatus(Status)
}

// Status defines the observed state of CustomObject.
// +k8s:deepcopy-gen=true
type Status struct {
	// State signifies current state of CustomObject.
	// Value can be one of ("Ready", "Processing", "Error", "Deleting").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Processing;Deleting;Ready;Error
	State State `json:"state,omitempty"`

	// Conditions contain a set of conditionals to determine the State of Status.
	// If all Conditions are met, the State is expected to be in StateReady.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Synced determine a list of Resources that are currently actively synced.
	// All resources that are synced are considered for orphan removal on configuration changes,
	// and it is used to determine effective differences from one state to the next.
	// +listType=atomic
	Synced        []Resource `json:"synced,omitempty"`
	LastOperation `json:"lastOperation,omitempty"`
}

type State string

// Valid States.
const (
	// StateReady signifies CustomObject is ready and has been installed successfully.
	StateReady State = "Ready"
	// StateProcessing signifies CustomObject is reconciling and is in the process of installation.
	// Processing can also signal that the Installation previously encountered an error and is now recovering.
	StateProcessing State = "Processing"
	// StateError signifies an error for CustomObject. This signifies that the Installation
	// process encountered an error.
	// Contrary to Processing, it can be expected that this state should change on the next retry.
	StateError State = "Error"
	// StateDeleting signifies CustomObject is being deleted. This is the state that is used
	// when a deletionTimestamp was detected and Finalizers are picked up.
	StateDeleting State = "Deleting"
)

func (s Status) WithState(state State) Status {
	s.State = state
	return s
}

func ResourcesDiff(resourcesA, resourcesB []Resource) []Resource {
	if len(resourcesA) < len(resourcesB) {
		return ResourcesDiff(resourcesB, resourcesA)
	}
	freqMap := make(map[string]struct{}, len(resourcesB))
	for _, x := range resourcesB {
		freqMap[x.ID()] = struct{}{}
	}
	var diff []Resource
	for _, x := range resourcesA {
		if _, found := freqMap[x.ID()]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

type Resource struct {
	Name                    string `json:"name"`
	Namespace               string `json:"namespace"`
	metav1.GroupVersionKind `json:",inline"`
}

func (r Resource) ToUnstructured() *unstructured.Unstructured {
	obj := unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind(r.GroupVersionKind))
	obj.SetName(r.Name)
	obj.SetNamespace(r.Namespace)
	return &obj
}

func (r Resource) ID() string {
	return strings.Join([]string{r.Namespace, r.Name, r.Group, r.Version, r.Kind}, "/")
}

// LastOperation defines the last operation from the control-loop.
// +k8s:deepcopy-gen=true
type LastOperation struct {
	Operation      string      `json:"operation"`
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
}

func (s Status) WithErr(err error) Status {
	s.LastOperation = LastOperation{Operation: err.Error(), LastUpdateTime: metav1.NewTime(time.Now())}
	return s
}

func (s Status) WithOperation(operation string) Status {
	s.LastOperation = LastOperation{Operation: operation, LastUpdateTime: metav1.NewTime(time.Now())}
	return s
}
