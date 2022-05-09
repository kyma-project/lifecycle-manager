package release

type Type string

const (
	Upgrade   Type = "upgrade"
	Downgrade Type = "downgrade"
	Install   Type = "install"
	Update    Type = "update"
)
