package watch

import (
	"context"
	"fmt"

	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type MandatoryTemplateChangeHandler struct {
	client.Reader
}

func NewMandatoryTemplateChangeHandler(handlerClient ChangeHandlerClient) *MandatoryTemplateChangeHandler {
	return &MandatoryTemplateChangeHandler{Reader: handlerClient}
}

func (h *MandatoryTemplateChangeHandler) Watch() handler.MapFunc {
	return func(ctx context.Context, o client.Object) []reconcile.Request {
		emptyRequest := make([]reconcile.Request, 0)
		template := &v1beta2.ModuleTemplate{}

		err := h.Get(ctx, client.ObjectKeyFromObject(o), template)
		if err != nil {
			return emptyRequest
		}

		if !template.Spec.Mandatory {
			return nil
		}

		kymas, err := getKymaList(ctx, h)
		if err != nil {
			return emptyRequest
		}

		return getRequestItems(kymas.Items)
	}
}

func getKymaList(ctx context.Context, clnt client.Reader) (*v1beta2.KymaList, error) {
	kymas := &v1beta2.KymaList{}
	listOptions := &client.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(k8slabels.Set{shared.ManagedBy: shared.OperatorName}),
	}
	err := clnt.List(ctx, kymas, listOptions)
	if err != nil {
		return kymas, fmt.Errorf("error in listing Kymas %w", err)
	}
	return kymas, nil
}

func getRequestItems(items []v1beta2.Kyma) []reconcile.Request {
	requests := make([]reconcile.Request, 0)
	for _, kyma := range items {
		requests = append(requests, reconcile.Request{NamespacedName: kyma.GetNamespacedName()})
	}
	return requests
}
