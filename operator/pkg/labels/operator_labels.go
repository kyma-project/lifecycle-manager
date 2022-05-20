package labels

const (
	OperatorPrefix  = "operator.kyma-project.io"
	ComponentPrefix = "component.kyma-project.io"
	Seperator       = "/"
	ControllerName  = OperatorPrefix + Seperator + "controller-name"
	Channel         = OperatorPrefix + Seperator + "channel"
	ManagedBy       = OperatorPrefix + Seperator + "managed-by"
)
