package labels

const (
	OperatorPrefix  = "operator.kyma-project.io"
	ComponentPrefix = "component.kyma-project.io"
	Seperator       = "/"
	ControllerName  = OperatorPrefix + Seperator + "controller-name"
	Release         = OperatorPrefix + Seperator + "release"
	AppliedAs       = OperatorPrefix + Seperator + "applied-as"
	ManagedBy       = OperatorPrefix + Seperator + "managed-by"
)
