package status_test

import (
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	errorsinternal "github.com/kyma-project/lifecycle-manager/internal/errors"
)

type skrClientRetrieverStub struct {
	client client.Client

	receivedKey types.NamespacedName
}

func (s *skrClientRetrieverStub) retrieverFunc() func(kymaName types.NamespacedName) (client.Client, error) {
	return func(kymaName types.NamespacedName) (client.Client, error) {
		s.receivedKey = kymaName
		if s.client == nil {
			return nil, errorsinternal.ErrSkrClientNotFound
		}
		return s.client, nil
	}
}
