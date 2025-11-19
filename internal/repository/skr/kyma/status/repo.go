package status

import (
	"context"
	"fmt"
	"time"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
	"github.com/kyma-project/lifecycle-manager/internal/errors"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
)

type SkrClientCache interface {
	Get(key client.ObjectKey) remote.Client
}

type Repository struct {
	skrClientCache SkrClientCache
}

func NewRepository(skrClientCache SkrClientCache) *Repository {
	return &Repository{
		skrClientCache: skrClientCache,
	}
}

func (r *Repository) Get(ctx context.Context, kymaName types.NamespacedName) (*v1beta2.KymaStatus, error) {
	skrClient, err := r.getSkrClient(kymaName)
	if err != nil {
		return nil, err
	}

	kyma := &v1beta2.Kyma{}
	if err := skrClient.Get(ctx,
		types.NamespacedName{
			Name:      shared.DefaultRemoteKymaName,
			Namespace: shared.DefaultRemoteNamespace,
		},
		kyma,
	); err != nil {
		return nil, err
	}

	return &kyma.Status, nil
}

func (r *Repository) SetStateDeleting(ctx context.Context, kymaName types.NamespacedName) error {
	skrClient, err := r.getSkrClient(kymaName)
	if err != nil {
		return err
	}

	kyma := &v1beta2.Kyma{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       string(shared.KymaKind),
			APIVersion: v1beta2.GroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      shared.DefaultRemoteKymaName,
			Namespace: shared.DefaultRemoteNamespace,
		},
		Status: v1beta2.KymaStatus{
			State: shared.StateDeleting,
			LastOperation: shared.LastOperation{
				Operation:      ".status.State set to Deleting",
				LastUpdateTime: apimetav1.NewTime(time.Now()),
			},
		},
	}

	return skrClient.Status().Patch(
		ctx,
		kyma,
		client.Apply,
		client.ForceOwnership,
		fieldowners.LifecycleManager,
	)
}

// TODO: this should work as long as we use the same client cache that we passed to KymaSkrContextProvider
// it however depends on KymaSkrContextProvider.Init being called. As of now, this is the case, but we
// should re-think how we use the client cache.
func (r *Repository) getSkrClient(kymaName types.NamespacedName) (client.Client, error) {
	skrClient := r.skrClientCache.Get(kymaName)

	if skrClient == nil {
		return nil, fmt.Errorf("%w: Kyma %s", errors.ErrSkrClientNotFound, kymaName.String())
	}

	return skrClient, nil
}
