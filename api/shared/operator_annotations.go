package shared

const (
	FQDN = OperatorGroup + Separator + "fqdn"

	// OwnedByAnnotation defines the resource managing the resource. Differing from ManagedBy
	// in that it does not reference controllers. Used by the runtime-watcher to determine the
	// corresponding CR in KCP.
	OwnedByAnnotation          = OperatorGroup + Separator + "owned-by"
	OwnedByFormat              = "%s/%s"
	SyncStrategyAnnotation     = "sync-strategy"
	IsClusterScopedAnnotation  = OperatorGroup + Separator + "is-cluster-scoped"
	CustomStateCheckAnnotation = OperatorGroup + Separator + "custom-state-check"
	ModuleVersionAnnotation    = OperatorGroup + Separator + "module-version"
)
