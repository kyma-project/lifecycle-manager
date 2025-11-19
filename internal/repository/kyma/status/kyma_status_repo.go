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

const (
	// TODO: discuss whether this message needs to be changed
	lastOperationDeleting = "waiting for modules to be deleted"
)

type Repository struct {
	statusWriter client.StatusWriter
}

func NewRepository(statusWriter client.StatusWriter) *Repository {
	return &Repository{
		statusWriter: statusWriter,
	}
}

func (r *Repository) UpdateStatusDeleting(ctx context.Context, kyma *v1beta2.Kyma) error {
	return r.updateKymaStatus(ctx, kyma, shared.StateDeleting, lastOperationDeleting)
}

func (r *Repository) updateKymaStatus(ctx context.Context,
	kyma *v1beta2.Kyma,
	newState shared.State,
	message string,
) error {
	kyma.Status.State = newState
	if newState == shared.StateReady || newState == shared.StateWarning {
		kyma.SetActiveChannel()
	}
	kyma.ManagedFields = nil
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
