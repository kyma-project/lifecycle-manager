package status_test

import (
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type skrClientCacheStub struct {
	client remote.Client

	receivedKey client.ObjectKey
}

func (s *skrClientCacheStub) Get(key client.ObjectKey) remote.Client {
	s.receivedKey = key
	return s.client
}
