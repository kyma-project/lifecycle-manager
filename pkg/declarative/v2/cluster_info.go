package v2

import (
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterInfo describes client and config for a cluster.
type ClusterInfo struct {
	Config *rest.Config
	Client client.Client
}

// IsEmpty indicates if ClusterInfo is empty.
func (r ClusterInfo) IsEmpty() bool {
	return r.Config == nil
}
