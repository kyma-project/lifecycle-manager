package v1beta1

import "github.com/kyma-project/lifecycle-manager/api/v1beta2"

const (
	FQDN = v1beta2.OperatorPrefix + v1beta2.Separator + "fqdn"

	// OwnedByAnnotation defines the resource managing the resource. Differing from ManagedBy
	// in that it does not reference controllers. Used by the runtime-watcher to determine the
	// corresponding CR in KCP.
	OwnedByAnnotation = v1beta2.OperatorPrefix + v1beta2.Separator + "owned-by"
	OwnedByFormat     = "%s/%s"
)
