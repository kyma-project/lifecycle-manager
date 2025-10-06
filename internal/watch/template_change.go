package watch

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type TemplateChangeHandler struct {
	client.Reader
}

func NewTemplateChangeHandler(handlerClient ChangeHandlerClient) *TemplateChangeHandler {
	return &TemplateChangeHandler{Reader: handlerClient}
}

func (h *TemplateChangeHandler) Watch() handler.MapFunc {
	return func(ctx context.Context, o client.Object) []reconcile.Request {
		emptyRequest := make([]reconcile.Request, 0)
		template := &v1beta2.ModuleTemplate{}

		if err := h.Get(ctx, client.ObjectKeyFromObject(o), template); err != nil {
			return emptyRequest
		}

		kymas, err := getKymaList(ctx, h)
		if err != nil {
			return emptyRequest
		}

		filteredKymas := filterKymasWithTemplate(kymas, template)

		return getRequestItems(filteredKymas)
	}
}

func filterKymasWithTemplate(kymas *v1beta2.KymaList, template *v1beta2.ModuleTemplate) []v1beta2.Kyma {
	items := make([]v1beta2.Kyma, 0, len(kymas.Items))
	for _, kyma := range kymas.Items {
		templateUsed := false
		for _, moduleStatus := range kyma.Status.Modules {
			if moduleStatus.Template == nil {
				continue
			}
			if moduleStatus.Template.GetName() == template.GetName() &&
				moduleStatus.Template.GetNamespace() == template.GetNamespace() &&
				moduleStatus.Channel != "" {
				templateUsed = true
				break
			}
		}
		if !templateUsed {
			continue
		}
		items = append(items, kyma)
	}

	return items
}
