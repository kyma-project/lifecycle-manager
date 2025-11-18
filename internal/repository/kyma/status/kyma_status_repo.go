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

type Repository struct {
	statusWriter client.StatusWriter
}

func NewRepository(statusWriter client.StatusWriter) *Repository {
	return &Repository{
		statusWriter: statusWriter,
	}
}

func (r *Repository) UpdateKymaStatus(ctx context.Context,
	kyma *v1beta2.Kyma,
	newState shared.State,
	message string,
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

	if err := r.statusWriter.Patch(ctx, kyma, client.Apply, &client.SubResourcePatchOptions{
		PatchOptions: *(&client.PatchOptions{}).ApplyOptions([]client.PatchOption{client.ForceOwnership}),
	}, client.FieldOwner(shared.OperatorName)); err != nil {
		return fmt.Errorf("status could not be updated: %w", err)
	}
	
	return nil
}
