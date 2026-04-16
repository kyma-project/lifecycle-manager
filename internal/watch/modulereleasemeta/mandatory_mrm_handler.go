package modulereleasemeta

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type mrmRepository interface {
	Get(ctx context.Context, name string) (*v1beta2.ModuleReleaseMeta, error)
}

type mandatoryKymaRepository interface {
	GetAll(ctx context.Context) (*v1beta2.KymaList, error)
}

// MandatoryMrmChangeHandler handles changes to mandatory ModuleReleaseMeta objects.
// Uses handler.MapFunc instead of a typed EventHandler: any change (create/update/delete)
// requeues all managed Kymas with no per-event distinction, so the typed machinery adds no value.
type MandatoryMrmChangeHandler struct {
	mrmRepository  mrmRepository
	kymaRepository mandatoryKymaRepository
}

func NewMandatoryMrmChangeHandler(mrmRepo mrmRepository, kymaRepo mandatoryKymaRepository) *MandatoryMrmChangeHandler {
	return &MandatoryMrmChangeHandler{
		mrmRepository:  mrmRepo,
		kymaRepository: kymaRepo,
	}
}

func (h *MandatoryMrmChangeHandler) Watch() handler.MapFunc {
	return func(ctx context.Context, o client.Object) []reconcile.Request {
		mrm, err := h.mrmRepository.Get(ctx, o.GetName())
		if err != nil {
			return nil
		}

		if mrm.Spec.Mandatory == nil {
			return nil
		}

		kymas, err := h.kymaRepository.GetAll(ctx)
		if err != nil {
			return nil
		}

		return toRequests(kymas.Items)
	}
}

func toRequests(kymas []v1beta2.Kyma) []reconcile.Request {
	requests := make([]reconcile.Request, 0, len(kymas))
	for _, kyma := range kymas {
		requests = append(requests, reconcile.Request{NamespacedName: kyma.GetNamespacedName()})
	}
	return requests
}
