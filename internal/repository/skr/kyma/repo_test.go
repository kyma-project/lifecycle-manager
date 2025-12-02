package kyma_test

import "sigs.k8s.io/controller-runtime/pkg/client"

type skrClientCacheStub struct {
	client client.Client

	receivedKey client.ObjectKey
}

func (s *skrClientCacheStub) Get(key client.ObjectKey) client.Client {
	s.receivedKey = key
	return s.client
}
