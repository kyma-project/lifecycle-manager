package modulereleasemeta

import (
	"context"
	"fmt"
	"reflect"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/errors"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var (
	mrmCrdName  = fmt.Sprintf("%s.%s", shared.ModuleReleaseMetaKind.Plural(), shared.OperatorGroup)
	crdTypeMeta = apimetav1.TypeMeta{
		Kind:       reflect.TypeOf(apiextensionsv1.CustomResourceDefinition{}).Name(),
		APIVersion: apiextensionsv1.SchemeGroupVersion.String(),
	}
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

func (r *Repository) CrdExists(ctx context.Context, kymaName types.NamespacedName) (bool, error) {
	skrClient, err := r.getSkrClient(kymaName)
	if err != nil {
		return false, err
	}

	err = skrClient.Get(ctx,
		types.NamespacedName{
			Name: mrmCrdName,
		},
		&v1beta1.PartialObjectMetadata{
			TypeMeta: crdTypeMeta,
		},
	)

	// not found error => (false, nil)
	// other error => (true, err)
	// no error => (true, nil)
	return !util.IsNotFound(err), client.IgnoreNotFound(err)
}

func (r *Repository) DeleteCrd(ctx context.Context, kymaName types.NamespacedName) error {
	skrClient, err := r.getSkrClient(kymaName)
	if err != nil {
		return err
	}

	return client.IgnoreNotFound(
		skrClient.Delete(ctx,
			&apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: apimetav1.ObjectMeta{
					Name: mrmCrdName,
				},
			}))
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
