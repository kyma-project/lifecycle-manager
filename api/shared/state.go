package shared

// +kubebuilder:validation:Enum=Processing;Deleting;Ready;Error;"";Warning

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

	// StateWarning signifies specified resource has been deployed, but cannot be used due to misconfiguration,
	// usually it means that user interaction is required.
	StateWarning State = "Warning"
)

// IsSupportedState These states will be used by module CR.
func (state State) IsSupportedState() bool {
	return state == StateReady ||
		state == StateProcessing ||
		state == StateError ||
		state == StateDeleting ||
		state == StateWarning
}

func AllStates() []State {
	return []State{StateReady, StateProcessing, StateError, StateDeleting, StateWarning}
}
