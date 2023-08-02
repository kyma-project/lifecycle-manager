package status

import (
	"context"
	"fmt"
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type KymaHelper struct {
	client.StatusWriter
	recordKymaStatusMetrics func(ctx context.Context, kyma *v1beta2.Kyma)
	isManagedKyma           bool
}

type HelperClient interface {
	Status() client.StatusWriter
	UpdateMetrics(ctx context.Context, kyma *v1beta2.Kyma)
	IsKymaManaged() bool
}

func Helper(handler HelperClient) *KymaHelper {
	return &KymaHelper{
		StatusWriter:            handler.Status(),
		recordKymaStatusMetrics: handler.UpdateMetrics,
		isManagedKyma:           handler.IsKymaManaged(),
	}
}

func (k *KymaHelper) UpdateStatusForExistingModules(ctx context.Context,
	kyma *v1beta2.Kyma, newState v1beta2.State, message string,
) error {
	kyma.Status.State = newState

	kyma.ManagedFields = nil

	switch newState {
	case v1beta2.StateReady, v1beta2.StateWarning:
		kyma.SetActiveChannel()
	case "":
	case v1beta2.StateDeleting:
	case v1beta2.StateError:
	case v1beta2.StateProcessing:
	default:
	}

	kyma.Status.LastOperation = v1beta2.LastOperation{
		Operation:      message,
		LastUpdateTime: metav1.NewTime(time.Now()),
	}

	fieldOwner := v1beta2.UnmanagedKyma
	if k.isManagedKyma {
		fieldOwner = v1beta2.OperatorName
	}
	if err := k.Patch(ctx, kyma, client.Apply, SubResourceOpts(client.ForceOwnership),
		client.FieldOwner(fieldOwner)); err != nil {
		return fmt.Errorf("status could not be updated: %w", err)
	}

	if k.recordKymaStatusMetrics != nil {
		k.recordKymaStatusMetrics(ctx, kyma)
	}

	return nil
}

func SubResourceOpts(opts ...client.PatchOption) client.SubResourcePatchOption {
	return &client.SubResourcePatchOptions{PatchOptions: *(&client.PatchOptions{}).ApplyOptions(opts)}
}
