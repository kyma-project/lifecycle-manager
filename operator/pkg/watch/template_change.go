package watch

import (
	"context"
	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"github.com/kyma-project/kyma-operator/operator/pkg/release/template"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type TemplateChangeHandler struct {
	Cache template.Cache
	client.Reader
	client.StatusWriter
	record.EventRecorder
}

func (h *TemplateChangeHandler) Watch(ctx context.Context) handler.MapFunc {
	logger := log.FromContext(ctx).WithName("template-change-detection")
	return func(o client.Object) []reconcile.Request {
		requests := make([]reconcile.Request, 0)
		moduleTemplate := &v1alpha1.ModuleTemplate{}

		if err := h.Get(ctx, client.ObjectKeyFromObject(o), moduleTemplate); err != nil {
			return requests
		}

		l := moduleTemplate.GetLabels()
		managedBy, managedByPresent := l[labels.ManagedBy]
		controller, controllerLabelPresent := l[labels.ControllerName]
		channel := moduleTemplate.Spec.Channel
		if !controllerLabelPresent || controller == "" ||
			channel == "" ||
			!managedByPresent || managedBy != "kyma-operator" {
			// limit cache from managedBy
			return requests
		}

		if err := h.Cache.Delete(template.GetTemplateCacheKey(controller, channel)); err != nil {
			panic(err)
		}

		kymas := &v1alpha1.KymaList{}
		err := h.List(ctx, kymas)
		if err != nil {
			return requests
		}

		templateNamespacedName := types.NamespacedName{
			Namespace: moduleTemplate.GetNamespace(),
			Name:      moduleTemplate.GetName(),
		}
		for _, kyma := range kymas.Items {
			globalChannelMatch := kyma.Spec.Channel == channel
			requeueKyma := false

			for _, component := range kyma.Spec.Components {
				if component.Name == controller {
					// check component level channel on matching component
					requeueKyma = (component.Channel == "" && globalChannelMatch) ||
						component.Channel == channel

					if requeueKyma {
						break
					}
				}
			}

			if !requeueKyma {
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
