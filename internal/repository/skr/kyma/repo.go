package kyma

import (
	"context"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type SkrClientRetrieverFunc func(kymaName types.NamespacedName) (client.Client, error)

type Repository struct {
	getSkrClient SkrClientRetrieverFunc
}

func NewRepository(getSkrClient SkrClientRetrieverFunc) *Repository {
	return &Repository{
		getSkrClient: getSkrClient,
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
	return !util.IsNotFound(err), util.IgnoreNotFound(err)
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

	return util.IgnoreNotFound(skrClient.Delete(ctx, kyma))
}
