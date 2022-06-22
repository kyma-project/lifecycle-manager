package labels

import "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"

const (
	SKREventsPrefix     = "skr.events.kyma-project.io"
	ListenerLastUpdated = SKREventsPrefix + Separator + "last-updated"
	ClusterName         = SKREventsPrefix + Separator + "cluster-name"
	OperatorPrefix      = "operator.kyma-project.io"
	ComponentPrefix     = "component.kyma-project.io"
	Separator           = "/"
	ControllerName      = OperatorPrefix + Separator + "controller-name"
	Channel             = OperatorPrefix + Separator + "channel"
	ManagedBy           = OperatorPrefix + Separator + "managed-by"
	Finalizer           = OperatorPrefix + Separator + v1alpha1.KymaKind
	ComponentOwner      = ComponentPrefix + Separator + "kyma-name"
)
