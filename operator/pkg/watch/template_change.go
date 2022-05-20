package watch

import (
	"context"

	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type TemplateChangeHandler struct {
	client.Reader
	client.StatusWriter
	record.EventRecorder
}

func (h *TemplateChangeHandler) Watch(ctx context.Context) handler.MapFunc {
	logger := log.FromContext(ctx).WithName("template-change-detection")
	return func(o client.Object) []reconcile.Request {
		requests := make([]reconcile.Request, 0)
		template := &v1alpha1.ModuleTemplate{}

		if err := h.Get(ctx, client.ObjectKeyFromObject(o), template); err != nil {
			return requests
		}

		l := template.GetLabels()
		managedBy, managedByPresent := l[labels.ManagedBy]
		controller, controllerLabelPresent := l[labels.ControllerName]
		channel := template.Spec.Channel
		if !controllerLabelPresent || controller == "" ||
			channel == "" ||
			!managedByPresent || managedBy != "kyma-operator" {
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
		for _, kyma := range kymas.Items {
			if kyma.Spec.Channel != v1alpha1.Channel(channel) {
				continue
			}

			referenced := false
			for _, component := range kyma.Spec.Components {
				if component.Name == controller {
					referenced = true
					break
				}
			}
			if !referenced {
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
