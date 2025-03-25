package repository

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type KymaRepository struct {
	client client.Client
}

func NewKymaRepository(client client.Client) *KymaRepository {
	return &KymaRepository{client: client}
}

func (r *KymaRepository) Get(ctx context.Context, namespacedName client.ObjectKey) (*v1beta2.Kyma, error) {
	kyma := &v1beta2.Kyma{}
	if err := r.client.Get(ctx, namespacedName, kyma); err != nil {
		return nil, fmt.Errorf("failed to get Kyma: %w", err)
	}
	return kyma, nil
}

func (r *KymaRepository) Update(ctx context.Context, kyma *v1beta2.Kyma) error {
	if err := r.client.Update(ctx, kyma); err != nil {
		return fmt.Errorf("failed to update Kyma: %w", err)
	}
	return nil
}

func (r *KymaRepository) StatusPatch(ctx context.Context, kyma *v1beta2.Kyma, fieldOwner string) error {
	if err := r.client.Status().Patch(ctx, kyma, client.Apply, SubResourceOpts(client.ForceOwnership),
		client.FieldOwner(fieldOwner)); err != nil {
		return fmt.Errorf("failed to update Kyma: %w", err)
	}
	return nil
}

func SubResourceOpts(opts ...client.PatchOption) *client.SubResourcePatchOptions {
	return &client.SubResourcePatchOptions{PatchOptions: *(&client.PatchOptions{}).ApplyOptions(opts)}
}
