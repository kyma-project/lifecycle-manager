package event

type eventReason string

type Event[T any] interface {
	Normal(object *T, reason eventReason)
	Warning(object *T, reason eventReason, err error)
}

const (
	typeNormal         = "Normal"
	typeWarning        = "Warning"
	maxErrorLength int = 50
)

func truncatedErrMsg(err error) string {
	msg := err.Error()
	length := len(msg)

	if length <= maxErrorLength {
		return msg
	}

	return msg[length-maxErrorLength:]
}
