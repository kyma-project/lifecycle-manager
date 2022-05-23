package util

import (
	"github.com/go-logr/logr"
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"github.com/kyma-project/kyma-operator/operator/pkg/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ComponentsAssociatedWithTemplate struct {
	ComponentName      string
	TemplateGeneration int64
	TemplateChannel    operatorv1alpha1.Channel
}

func SetComponentCRLabels(unstructuredCompCR *unstructured.Unstructured, componentName string, channel operatorv1alpha1.Channel) {
	labelMap := unstructuredCompCR.GetLabels()
	if labelMap == nil {
		labelMap = make(map[string]string)
	}
	labelMap[labels.ControllerName] = componentName
	labelMap[labels.Channel] = string(channel)
	unstructuredCompCR.SetLabels(labelMap)
}

func AreTemplatesOutdated(logger *logr.Logger, k *operatorv1alpha1.Kyma, lookupResults release.TemplateLookupResultsByName) bool {
	// this is a shortcut as we already know templates are outdated when the generation changes
	if k.GetGeneration() != k.Status.ObservedGeneration {
		logger.Info("new kyma spec, setting template status outdated")
		return true
	}
	// in the case that the kyma spec did not change, we only have to verify that all desired templates are still referenced in the latest spec generation
	for componentName, lookupResult := range lookupResults {
		for _, condition := range k.Status.Conditions {
			if condition.Reason == componentName && lookupResult != nil {
				if lookupResult.Template.GetGeneration() != condition.TemplateInfo.Generation {
					logger.Info("detected outdated template",
						"condition", condition.Reason,
						"template", lookupResult.Template.Name,
						"templateGeneration", lookupResult.Template.GetGeneration(),
						"previousGeneration", condition.TemplateInfo.Generation,
						"templateChannel", lookupResult.Template.Spec.Channel,
						"previousChannel", condition.TemplateInfo.Channel,
					)
					return true
				}

			}
		}
	}
	return false
}

func CopyComponentSettingsToUnstructuredFromResource(resource *unstructured.Unstructured, component operatorv1alpha1.ComponentType) {
	if len(component.Settings) > 0 {
		var charts []map[string]interface{}
		for _, setting := range component.Settings {
			chart := map[string]interface{}{}
			for key, value := range setting {
				chart[key] = value
			}
			charts = append(charts, chart)
		}
		resource.Object["spec"].(map[string]interface{})["charts"] = charts
	}
}
