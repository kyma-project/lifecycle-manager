package crd

import (
	"context"
	"reflect"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type SkrClientRetrieverFunc func(kymaName types.NamespacedName) (client.Client, error)

type Repository struct {
	getSkrClient SkrClientRetrieverFunc
	crdName      string
}

func NewRepository(getSkrClient SkrClientRetrieverFunc,
	crdName string,
) *Repository {
	return &Repository{
		getSkrClient: getSkrClient,
		crdName:      crdName,
	}
}

func (r *Repository) Exists(ctx context.Context, kymaName types.NamespacedName) (bool, error) {
	skrClient, err := r.getSkrClient(kymaName)
	if err != nil {
		return false, err
	}

	err = skrClient.Get(ctx,
		types.NamespacedName{
			Name: r.crdName,
		},
		&v1beta1.PartialObjectMetadata{
			TypeMeta: apimetav1.TypeMeta{
				Kind:       reflect.TypeFor[apiextensionsv1.CustomResourceDefinition]().Name(),
				APIVersion: apiextensionsv1.SchemeGroupVersion.String(),
			},
		},
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

	return util.IgnoreNotFound(
		skrClient.Delete(ctx,
			&apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: apimetav1.ObjectMeta{
					Name: r.crdName,
				},
			}))
}
