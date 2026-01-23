package status

import (
	"context"
	"fmt"
	"time"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type KymaHelper struct {
	client.StatusWriter

	recordKymaStatusMetrics func(ctx context.Context, kyma *v1beta2.Kyma)
}

type HelperClient interface {
	Status() client.StatusWriter
	UpdateMetrics(ctx context.Context, kyma *v1beta2.Kyma)
}

func Helper(handler HelperClient) *KymaHelper {
	return &KymaHelper{
		StatusWriter:            handler.Status(),
		recordKymaStatusMetrics: handler.UpdateMetrics,
	}
}

func (k *KymaHelper) UpdateStatusForExistingModules(ctx context.Context,
	kyma *v1beta2.Kyma, newState shared.State, message string,
) error {
	kyma.Status.State = newState
	kyma.ManagedFields = nil

	switch newState {
	case shared.StateReady, shared.StateWarning:
		kyma.SetActiveChannel()
	case "":
	case shared.StateDeleting:
	case shared.StateError:
	case shared.StateProcessing:
	case shared.StateUnmanaged:
	default:
	}

	kyma.Status.LastOperation = shared.LastOperation{
		Operation:      message,
		LastUpdateTime: apimetav1.NewTime(time.Now()),
	}

	fieldOwner := shared.OperatorName
	if err := k.Patch(ctx, kyma, client.Apply, SubResourceOpts(client.ForceOwnership),
		client.FieldOwner(fieldOwner)); err != nil {
		return fmt.Errorf("status could not be updated: %w", err)
	}

	if k.recordKymaStatusMetrics != nil {
		k.recordKymaStatusMetrics(ctx, kyma)
	}

	return nil
}

func SubResourceOpts(opts ...client.PatchOption) *client.SubResourcePatchOptions {
	return &client.SubResourcePatchOptions{PatchOptions: *(&client.PatchOptions{}).ApplyOptions(opts)}
}
