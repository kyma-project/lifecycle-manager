package shared

// SyncStrategy determines how the Remote Cluster is synchronized with the Control Plane. This can influence secret
// lookup, or other behavioral patterns when interacting with the remote cluster.
type SyncStrategy string

const (
	SyncStrategyLocalSecret = "local-secret"
	SyncStrategyLocalClient = "local-client"
)
