package watch

import (
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ChangeHandlerClient interface {
	client.Reader
	record.EventRecorder
}
