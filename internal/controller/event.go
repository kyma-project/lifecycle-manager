package controller

type eventReason string

type Event[T any] interface {
	Normal(object *T, reason eventReason)
	Warning(object *T, reason eventReason, err error)
}

const (
	typeNormal  = "Normal"
)
