package status

import (
	"context"
	"fmt"
	"time"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/repository"
)

type KymaHelper struct {
	kymaRepository          *repository.KymaRepository
	recordKymaStatusMetrics func(ctx context.Context, kyma *v1beta2.Kyma)
	isManagedKyma           bool
}

type HelperClient interface {
	UpdateMetrics(ctx context.Context, kyma *v1beta2.Kyma)
	IsKymaManaged() bool
}

func Helper(kymaRepository *repository.KymaRepository, handler HelperClient) *KymaHelper {
	return &KymaHelper{
		kymaRepository:          kymaRepository,
		recordKymaStatusMetrics: handler.UpdateMetrics,
		isManagedKyma:           handler.IsKymaManaged(),
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

	fieldOwner := shared.UnmanagedKyma
	if k.isManagedKyma {
		fieldOwner = shared.OperatorName
	}
	if err := k.kymaRepository.StatusPatch(ctx, kyma, fieldOwner); err != nil {
		return fmt.Errorf("status could not be updated: %w", err)
	}

	if k.isManagedKyma && k.recordKymaStatusMetrics != nil {
		k.recordKymaStatusMetrics(ctx, kyma)
	}

	return nil
}

func SubResourceOpts(opts ...client.PatchOption) *client.SubResourcePatchOptions {
	return &client.SubResourcePatchOptions{PatchOptions: *(&client.PatchOptions{}).ApplyOptions(opts)}
}
