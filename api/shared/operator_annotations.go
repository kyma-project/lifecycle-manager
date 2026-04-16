package shared

const (
	FQDN             = OperatorGroup + Separator + "fqdn" // Deprecated: use OCMComponentName instead.
	OCMComponentName = OperatorGroup + Separator + "ocm-component-name"

	// OwnedByAnnotation defines the resource managing the resource. Differing from ManagedBy
	// in that it does not reference controllers. Used by the runtime-watcher to determine the
	// corresponding CR in KCP.
	OwnedByAnnotation         = "deprecated-" + OperatorGroup + Separator + "owned-by"
	OwnedByFormat             = "%s/%s"
	IsClusterScopedAnnotation = OperatorGroup + Separator + "is-cluster-scoped"
	UnmanagedAnnotation       = OperatorGroup + Separator + "is-unmanaged"
)
