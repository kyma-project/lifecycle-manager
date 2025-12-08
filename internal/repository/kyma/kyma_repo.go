package kyma

import (
	"context"
	"fmt"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type Repository struct {
	client    client.Client
	namespace string
}

func NewRepository(client client.Client, namespace string) *Repository {
	return &Repository{
		client:    client,
		namespace: namespace,
	}
}

func (r *Repository) Get(ctx context.Context, kymaName string) (*v1beta2.Kyma, error) {
	kyma := &v1beta2.Kyma{}
	err := r.client.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: r.namespace}, kyma)
	if err != nil {
		return nil, fmt.Errorf("failed to get Kyma %s in namespace %s: %w", kymaName, r.namespace, err)
	}
	return kyma, nil
}

func (r *Repository) DropAllFinalizers(ctx context.Context, kymaName string) error {
	pom := &v1beta1.PartialObjectMetadata{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       string(shared.KymaKind),
			APIVersion: v1beta2.GroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      kymaName,
			Namespace: r.namespace,
		},
	}

	// SSA would not work here since currently there are multiple field managers
	// on the finalizers
	patch := []byte(`{"metadata":{"finalizers":[]}}`)
	err := r.client.Patch(ctx,
		pom,
		client.RawPatch(client.Merge.Type(), patch),
	)

	return util.IgnoreNotFound(err)
}
