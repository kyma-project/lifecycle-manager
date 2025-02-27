package repository

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type KymaRepository struct {
	Client client.Client
}

func NewKymaRepository(client client.Client) *KymaRepository {
	return &KymaRepository{Client: client}
}

func (r *KymaRepository) GetKyma(ctx context.Context, namespacedName client.ObjectKey) (*v1beta2.Kyma, error) {
	kyma := &v1beta2.Kyma{}
	if err := r.Client.Get(ctx, namespacedName, kyma); err != nil {
		return nil, fmt.Errorf("failed to get Kyma: %w", err)
	}
	return kyma, nil
}
