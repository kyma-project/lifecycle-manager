package remote

import (
	"github.com/prometheus/prometheus/storage/remote"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type ClientFactory interface {
	GetClient(kyma *v1beta2.Kyma, syncNamespace string) (remote.Client, error)
}
