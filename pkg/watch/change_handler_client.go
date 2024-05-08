package watch

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ChangeHandlerClient interface {
	client.Reader
}
