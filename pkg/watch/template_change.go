package watch

import (
	"context"

	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type TemplateChangeHandler struct {
	client.Reader
	record.EventRecorder
	NamespaceScoped bool
}

func NewTemplateChangeHandler(handlerClient ChangeHandlerClient) *TemplateChangeHandler {
	return &TemplateChangeHandler{Reader: handlerClient, EventRecorder: handlerClient, NamespaceScoped: false}
}

func (h *TemplateChangeHandler) Watch() handler.MapFunc {
	return func(ctx context.Context, o client.Object) []reconcile.Request {
		requests := make([]reconcile.Request, 0)
		template := &v1beta2.ModuleTemplate{}

		if err := h.Get(ctx, client.ObjectKeyFromObject(o), template); err != nil {
			return requests
		}

		kymas := &v1beta2.KymaList{}
		listOptions := &client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(k8slabels.Set{shared.ManagedBy: shared.OperatorName}),
		}
		if h.NamespaceScoped {
			listOptions.Namespace = template.Namespace
		}
		err := h.List(ctx, kymas, listOptions)
		if err != nil {
			return requests
		}

		for _, kyma := range kymas.Items {
			templateUsed := false
			for _, moduleStatus := range kyma.Status.Modules {
				if moduleStatus.Template == nil {
					continue
				}
				if moduleStatus.Template.GetName() == template.GetName() &&
					moduleStatus.Template.GetNamespace() == template.GetNamespace() {
					templateUsed = true
					break
				}
			}
			if !templateUsed {
				return nil
			}
			
			kymaName := types.NamespacedName{
				Namespace: kyma.GetNamespace(),
				Name:      kyma.GetName(),
			}

			requests = append(requests, reconcile.Request{NamespacedName: kymaName})
		}

		return requests
	}
}
