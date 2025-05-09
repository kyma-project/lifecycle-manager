package kyma

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type Client struct {
	client.Reader
}

func NewClient(reader client.Reader) *Client {
	return &Client{
		Reader: reader,
	}
}

func (c *Client) GetKyma(ctx context.Context, kymaName string, kymaNamespace string) (*v1beta2.Kyma, error) {
	kyma := &v1beta2.Kyma{}
	err := c.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, kyma)
	if err != nil {
		return nil, fmt.Errorf("failed to get Kyma %s in namespace %s: %w", kymaName, kymaNamespace, err)
	}
	return kyma, nil
}
