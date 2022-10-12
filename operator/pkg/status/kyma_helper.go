package status

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
)

type KymaHelper struct {
	client.StatusWriter
}

type HelperClient interface {
	Status() client.StatusWriter
}

func Helper(handler HelperClient) *KymaHelper {
	return &KymaHelper{StatusWriter: handler.Status()}
}

func (k *KymaHelper) UpdateStatusForExistingModules(ctx context.Context,
	kyma *v1alpha1.Kyma, newState v1alpha1.State,
) error {
	kyma.Status.State = newState

	switch newState {
	case v1alpha1.StateReady:
		kyma.SetActiveChannel()
	case "":
	case v1alpha1.StateDeleting:
	case v1alpha1.StateError:
	case v1alpha1.StateProcessing:
	default:
	}

	if err := k.Update(ctx, kyma); err != nil {
		return fmt.Errorf("status could not be updated: %w", err)
	}

	return nil
}
