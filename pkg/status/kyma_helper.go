package status

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
)

type KymaHelper struct {
	client.StatusWriter
	recordKymaStatusMetrics func(ctx context.Context, kyma *v1beta1.Kyma)
	isManagedKyma           bool
}

type HelperClient interface {
	Status() client.StatusWriter
	RecordKymaStatusMetrics(ctx context.Context, kyma *v1beta1.Kyma)
	DetermineIsKymaManaged() bool
}

func Helper(handler HelperClient) *KymaHelper {
	return &KymaHelper{
		StatusWriter:            handler.Status(),
		recordKymaStatusMetrics: handler.RecordKymaStatusMetrics,
		isManagedKyma:           handler.DetermineIsKymaManaged(),
	}
}

func (k *KymaHelper) UpdateStatusForExistingModules(ctx context.Context,
	kyma *v1beta1.Kyma, newState v1beta1.State, message string,
) error {
	kyma.Status.State = newState

	kyma.ManagedFields = nil

	switch newState {
	case v1beta1.StateReady:
		kyma.SetActiveChannel()
	case "":
	case v1beta1.StateDeleting:
	case v1beta1.StateError:
	case v1beta1.StateProcessing:
	default:
	}

	kyma.Status.LastOperation = v1beta1.LastOperation{
		Operation:      message,
		LastUpdateTime: metav1.NewTime(time.Now()),
	}

	fieldOwner := v1beta1.UnmanagedKyma
	if k.isManagedKyma {
		fieldOwner = v1beta1.OperatorName
	}
	if err := k.Patch(ctx, kyma, client.Apply, subResourceOpts(client.ForceOwnership),
		client.FieldOwner(fieldOwner)); err != nil {
		return fmt.Errorf("status could not be updated: %w", err)
	}

	if k.recordKymaStatusMetrics != nil {
		k.recordKymaStatusMetrics(ctx, kyma)
	}

	return nil
}

func subResourceOpts(opts ...client.PatchOption) client.SubResourcePatchOption {
	return &client.SubResourcePatchOptions{PatchOptions: *(&client.PatchOptions{}).ApplyOptions(opts)}
}
