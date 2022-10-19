package v1alpha1

const (
	LastSync                       = OperatorPrefix + Separator + "last-sync"
	LastSyncGenerationControlPlane = OperatorPrefix + Separator + "last-sync-gen-control-plane"
	LastSyncGenerationRuntime      = OperatorPrefix + Separator + "last-sync-gen-runtime"
)
