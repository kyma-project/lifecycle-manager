package event

import "k8s.io/apimachinery/pkg/runtime"

type Event interface {
	Normal(object runtime.Object, reason string)
	Warning(object runtime.Object, reason string, err error)
}
