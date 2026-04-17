package watch

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type kymaRepository interface {
	LookupByLabel(ctx context.Context, labelName, labelValue string) (*v1beta2.KymaList, error)
}

// TemplateChangeHandler handles changes to ModuleTemplate objects.
// Uses handler.MapFunc instead of a typed EventHandler: any change (create/update/delete)
// requeues all Kymas referencing the template with no per-event distinction, so the typed machinery adds no value.
type TemplateChangeHandler struct {
	kymaRepository kymaRepository
}

func NewTemplateChangeHandler(kymaRepo kymaRepository) *TemplateChangeHandler {
	return &TemplateChangeHandler{
		kymaRepository: kymaRepo,
	}
}

func (h *TemplateChangeHandler) Watch() handler.MapFunc {
	return func(ctx context.Context, o client.Object) []reconcile.Request {
		template, ok := o.(*v1beta2.ModuleTemplate)
		if !ok {
			return nil
		}

		kymas, err := h.kymaRepository.LookupByLabel(ctx, shared.ManagedBy, shared.OperatorName)
		if err != nil {
			return nil
		}

		return getRequestItems(filterKymasWithTemplate(kymas, template))
	}
}

func getRequestItems(kymas []v1beta2.Kyma) []reconcile.Request {
	requests := make([]reconcile.Request, 0, len(kymas))
	for _, kyma := range kymas {
		requests = append(requests, reconcile.Request{NamespacedName: kyma.GetNamespacedName()})
	}
	return requests
}

func filterKymasWithTemplate(kymas *v1beta2.KymaList, template *v1beta2.ModuleTemplate) []v1beta2.Kyma {
	items := make([]v1beta2.Kyma, 0, len(kymas.Items))
	for _, kyma := range kymas.Items {
		for _, moduleStatus := range kyma.Status.Modules {
			if moduleStatus.Template == nil {
				continue
			}
			if moduleStatus.Template.GetName() == template.GetName() &&
				moduleStatus.Template.GetNamespace() == template.GetNamespace() &&
				moduleStatus.Channel != "" {
				items = append(items, kyma)
				break
			}
		}
	}
	return items
}
