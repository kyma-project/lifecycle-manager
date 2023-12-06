package shared

import "github.com/kyma-project/lifecycle-manager/api/v1beta2"

const (
	FQDN = v1beta2.OperatorPrefix + v1beta2.Separator + "fqdn"

	// OwnedByAnnotation defines the resource managing the resource. Differing from ManagedBy
	// in that it does not reference controllers. Used by the runtime-watcher to determine the
	// corresponding CR in KCP.
	OwnedByAnnotation          = v1beta2.OperatorPrefix + v1beta2.Separator + "owned-by"
	OwnedByFormat              = "%s/%s"
	SyncStrategyAnnotation     = "sync-strategy"
	IsClusterScopedAnnotation  = v1beta2.OperatorPrefix + v1beta2.Separator + "is-cluster-scoped"
	CustomStateCheckAnnotation = v1beta2.OperatorPrefix + v1beta2.Separator + "custom-state-check"
	ModuleVersionAnnotation    = v1beta2.OperatorPrefix + "module-version"
)
