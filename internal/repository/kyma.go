package repository

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type KymaRepository struct {
	Client client.Client
}

func (r *KymaRepository) GetKyma(ctx context.Context, namespacedName client.ObjectKey) (*v1beta2.Kyma, error) {
	kyma := &v1beta2.Kyma{}
	if err := r.Client.Get(ctx, namespacedName, kyma); err != nil {
		return nil, err
	}
	return kyma, nil
}
