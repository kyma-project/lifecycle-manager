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
}

type HelperClient interface {
	Status() client.StatusWriter
}

func Helper(handler HelperClient) *KymaHelper {
	return &KymaHelper{StatusWriter: handler.Status()}
}

func (k *KymaHelper) UpdateStatusForExistingModules(ctx context.Context,
	kyma *v1beta1.Kyma, newState v1beta1.State, message string,
) error {
	kyma.Status.State = newState

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

	if err := k.Update(ctx, kyma); err != nil {
		return fmt.Errorf("status could not be updated: %w", err)
	}

	return nil
}
