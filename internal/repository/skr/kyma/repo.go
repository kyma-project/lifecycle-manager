package kyma

import (
	"context"
	"fmt"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/errors"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type SkrClientCache interface {
	Get(key client.ObjectKey) client.Client
}

type Repository struct {
	skrClientCache SkrClientCache
}

func NewRepository(skrClientCache SkrClientCache) *Repository {
	return &Repository{
		skrClientCache: skrClientCache,
	}
}

func (r *Repository) Exists(ctx context.Context, kymaName types.NamespacedName) (bool, error) {
	skrClient, err := r.getSkrClient(kymaName)
	if err != nil {
		return false, err
	}

	pom := &v1beta1.PartialObjectMetadata{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       string(shared.KymaKind),
			APIVersion: v1beta2.GroupVersion.String(),
		},
	}

	err = skrClient.Get(ctx,
		types.NamespacedName{
			Name:      shared.DefaultRemoteKymaName,
			Namespace: shared.DefaultRemoteNamespace,
		},
		pom,
	)

	// not found error => (false, nil)
	// other error => (true, err)
	// no error => (true, nil)
	return !util.IsNotFound(err), client.IgnoreNotFound(err)
}

func (r *Repository) Delete(ctx context.Context, kymaName types.NamespacedName) error {
	skrClient, err := r.getSkrClient(kymaName)
	if err != nil {
		return err
	}

	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      shared.DefaultRemoteKymaName,
			Namespace: shared.DefaultRemoteNamespace,
		},
	}

	return client.IgnoreNotFound(skrClient.Delete(ctx, kyma))
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
