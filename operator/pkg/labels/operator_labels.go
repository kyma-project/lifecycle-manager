package labels

const (
	Prefix         = "operator.kyma-project.io"
	Seperator      = "/"
	ControllerName = Prefix + Seperator + "controller-name"
	Release        = Prefix + Seperator + "release"
	AppliedAs      = Prefix + Seperator + "applied-as"
	ManagedBy      = Prefix + Seperator + "managed-by"
)
