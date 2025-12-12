package status

import (
	"context"
	"errors"
	"fmt"
	"time"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
)

const operationSetStateDeleting = ".status.State set to Deleting"

var errStatusPatchFailed = errors.New("status patch failed")

type SkrClientRetrieverFunc func(kymaName types.NamespacedName) (client.Client, error)

type Repository struct {
	getSkrClient SkrClientRetrieverFunc
}

func NewRepository(getSkrClient SkrClientRetrieverFunc) *Repository {
	return &Repository{
		getSkrClient: getSkrClient,
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
		return nil, fmt.Errorf("failed to get Kyma CR from SKR: %w", err)
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
				Operation:      operationSetStateDeleting,
				LastUpdateTime: apimetav1.NewTime(time.Now()),
			},
		},
	}

	if err := skrClient.Status().Patch(
		ctx,
		kyma,
		client.Apply,
		client.ForceOwnership,
		fieldowners.LifecycleManager,
	); err != nil {
		return errors.Join(err, errStatusPatchFailed)
	}

	return nil
}
