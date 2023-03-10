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
	recordKymaStatusMetrics func(kyma *v1beta1.Kyma)
}

type HelperClient interface {
	Status() client.StatusWriter
	RecordKymaStatusMetrics(kyma *v1beta1.Kyma)
}

func Helper(handler HelperClient) *KymaHelper {
	return &KymaHelper{
		StatusWriter:            handler.Status(),
		recordKymaStatusMetrics: handler.RecordKymaStatusMetrics,
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

	if err := k.Patch(ctx, kyma, client.Apply, subResourceOpts(client.ForceOwnership),
		client.FieldOwner(v1beta1.OperatorName)); err != nil {
		return fmt.Errorf("status could not be updated: %w", err)
	}

	if k.recordKymaStatusMetrics != nil {
		k.recordKymaStatusMetrics(kyma)
	}

	return nil
}

func subResourceOpts(opts ...client.PatchOption) client.SubResourcePatchOption {
	return &client.SubResourcePatchOptions{PatchOptions: *(&client.PatchOptions{}).ApplyOptions(opts)}
}
