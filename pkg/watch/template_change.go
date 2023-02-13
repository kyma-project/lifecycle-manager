package watch

import (
	"context"

	k8slabels "k8s.io/apimachinery/pkg/labels"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type TemplateChangeHandler struct {
	client.Reader
	record.EventRecorder
	NamespaceScoped bool
}

func NewTemplateChangeHandler(handlerClient ChangeHandlerClient) *TemplateChangeHandler {
	return &TemplateChangeHandler{Reader: handlerClient, EventRecorder: handlerClient, NamespaceScoped: false}
}

func (h *TemplateChangeHandler) Watch(ctx context.Context) handler.MapFunc {
	return func(o client.Object) []reconcile.Request {
		requests := make([]reconcile.Request, 0)
		template := &v1beta1.ModuleTemplate{}

		if err := h.Get(ctx, client.ObjectKeyFromObject(o), template); err != nil {
			return requests
		}

		if !manageable(template) {
			return requests
		}

		kymas := &v1beta1.KymaList{}
		listOptions := &client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(k8slabels.Set{v1beta1.ManagedBy: v1beta1.OperatorName}),
		}
		if h.NamespaceScoped {
			listOptions.Namespace = template.Namespace
		}
		err := h.List(ctx, kymas, listOptions)
		if err != nil {
			return requests
		}

		logger := log.FromContext(ctx)

		for _, kyma := range kymas.Items {
			templateUsed := false
			for _, moduleStatus := range kyma.Status.Modules {
				if moduleStatus.Template.GetName() == template.GetName() &&
					moduleStatus.Template.GetNamespace() == template.GetNamespace() {
					templateUsed = true
					break
				}
			}
			if !templateUsed {
				return nil
			}

			templateName := types.NamespacedName{
				Namespace: template.GetNamespace(),
				Name:      template.GetName(),
			}
			kymaName := types.NamespacedName{
				Namespace: kyma.GetNamespace(),
				Name:      kyma.GetName(),
			}

			logger.WithValues("template", templateName.String(), "kyma", kymaName.String()).Info(
				"Kyma CR instance is scheduled for reconciliation because a relevant ModuleTemplate changed",
			)

			requests = append(requests, reconcile.Request{NamespacedName: kymaName})
		}

		return requests
	}
}

func manageable(template *v1beta1.ModuleTemplate) bool {
	labels := template.GetLabels()

	if managedBy, ok := labels[v1beta1.ManagedBy]; !ok || managedBy != v1beta1.OperatorName {
		return false
	}
	if controller, ok := labels[v1beta1.ControllerName]; !ok || controller == "" {
		return false
	}
	if template.Spec.Target == v1beta1.TargetControlPlane || template.Spec.Channel == "" {
		return false
	}
	return true
}

func requeueKyma(kyma v1beta1.Kyma, moduleName, templateChannel string) bool {
	globalChannelMatch := kyma.Spec.Channel == templateChannel

	for _, module := range kyma.Spec.Modules {
		if module.Name == moduleName {
			// check module level channel on matching module
			if (module.Channel == "" && globalChannelMatch) ||
				module.Channel == templateChannel {
				return true
			}
		}
	}

	return false
}
