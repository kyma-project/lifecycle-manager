package labels

import "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"

const (
	OperatorPrefix      = "operator.kyma-project.io"
	ComponentPrefix     = "component.kyma-project.io"
	SKREventsPrefix     = "skr.events.kyma-project.io"
	Separator           = "/"
	ControllerName      = OperatorPrefix + Separator + "controller-name"
	Channel             = OperatorPrefix + Separator + "channel"
	ManagedBy           = OperatorPrefix + Separator + "managed-by"
	Finalizer           = OperatorPrefix + Separator + v1alpha1.KymaKind
	ListenerLastUpdated = SKREventsPrefix + Separator + "last-updated"
	ClusterName         = SKREventsPrefix + Separator + "cluster-name"
)
