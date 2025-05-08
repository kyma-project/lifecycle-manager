package orphan

import (
	"context"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type KymaClient struct {
	client.Reader
}

func NewKymaClient(reader client.Reader) *KymaClient {
	return &KymaClient{
		Reader: reader,
	}
}

func (c *KymaClient) GetKyma(ctx context.Context, kymaName string, namespace string) (*v1beta2.Kyma, error) {
	kyma := &v1beta2.Kyma{}
	err := c.Reader.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: namespace}, kyma)
	if err != nil {
		return nil, err
	}
	return kyma, nil
}
