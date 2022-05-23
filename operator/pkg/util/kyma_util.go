package util

import (
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
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
