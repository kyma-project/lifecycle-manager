package watch

import (
	"context"

	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
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
}

func NewTemplateChangeHandler(handlerClient ChangeHandlerClient) *TemplateChangeHandler {
	return &TemplateChangeHandler{Reader: handlerClient, EventRecorder: handlerClient}
}

func (h *TemplateChangeHandler) Watch(ctx context.Context) handler.MapFunc {
	return func(o client.Object) []reconcile.Request {
		requests := make([]reconcile.Request, 0)
		template := &v1alpha1.ModuleTemplate{}

		if err := h.Get(ctx, client.ObjectKeyFromObject(o), template); err != nil {
			return requests
		}

		if !manageable(template) {
			return requests
		}

		kymas := &v1alpha1.KymaList{}
		err := h.List(ctx, kymas)
		if err != nil {
			return requests
		}

		templateNamespacedName := types.NamespacedName{
			Namespace: template.GetNamespace(),
			Name:      template.GetName(),
		}
		logger := log.FromContext(ctx).WithName("template-change-detection")

		labels := template.GetLabels()
		controller := labels[v1alpha1.ControllerName]
		channel := template.Spec.Channel

		for _, kyma := range kymas.Items {
			globalChannelMatch := kyma.Spec.Channel == channel

			if !requeueKyma(kyma, controller, globalChannelMatch, channel) {
				continue
			}

			namespacedNameForKyma := types.NamespacedName{
				Namespace: kyma.GetNamespace(),
				Name:      kyma.GetName(),
			}
			logger.WithValues(
				"controller", controller,
				"channel", channel,
				"template", templateNamespacedName.String(),
			).Info(namespacedNameForKyma.String())

			requests = append(requests, reconcile.Request{NamespacedName: namespacedNameForKyma})
		}

		return requests
	}
}

func manageable(template *v1alpha1.ModuleTemplate) bool {
	labels := template.GetLabels()

	if managedBy, ok := labels[v1alpha1.ManagedBy]; !ok || managedBy != v1alpha1.OperatorName {
		return false
	}
	if controller, ok := labels[v1alpha1.ControllerName]; !ok || controller == "" {
		return false
	}
	if template.Spec.Target == v1alpha1.TargetControlPlane || template.Spec.Channel == "" {
		return false
	}
	return true
}

func requeueKyma(kyma v1alpha1.Kyma, controller string, globalChannelMatch bool, channel v1alpha1.Channel) bool {
	for _, component := range kyma.Spec.Modules {
		if component.Name == controller {
			// check component level channel on matching component
			if (component.Channel == "" && globalChannelMatch) ||
				component.Channel == channel {
				return true
			}
		}
	}

	return false
}
