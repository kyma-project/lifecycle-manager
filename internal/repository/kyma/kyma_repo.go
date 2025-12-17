package kyma

import (
	"context"
	"encoding/json"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
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

func (r *Repository) LookupByLabel(ctx context.Context, labelKey, labelValue string) (*v1beta2.KymaList, error) {
	kymaList := &v1beta2.KymaList{}
	if err := r.client.List(ctx, kymaList, client.InNamespace(r.namespace),
		client.MatchingLabels{labelKey: labelValue}); err != nil {
		return nil, fmt.Errorf("failed to list Kymas in namespace %s with label %s=%s: %w",
			r.namespace, labelKey, labelValue, err)
	}

	return kymaList, nil
}

func (r *Repository) DropFinalizer(ctx context.Context, kymaName string, finalizer string) error {
	kyma, err := r.Get(ctx, kymaName)
	if err != nil {
		return util.IgnoreNotFound(fmt.Errorf("failed to get current finalizers: %w", err))
	}

	if !controllerutil.RemoveFinalizer(kyma, finalizer) {
		return nil
	}

	remainingFinalizers, err := json.Marshal(kyma.Finalizers)
	if err != nil {
		return fmt.Errorf("failed marshal remaining finalizers: %w", err)
	}

	// SSA would not work here since currently there are multiple field managers
	// on the finalizers
	patch := fmt.Appendf(nil, `{"metadata":{"finalizers":%s}}`, remainingFinalizers)
	return util.IgnoreNotFound(
		r.client.Patch(ctx,
			kyma,
			client.RawPatch(client.Merge.Type(), patch),
			fieldowners.LifecycleManager,
		),
	)
}
