package util

import (
	"fmt"

	"github.com/go-logr/logr"
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"github.com/kyma-project/kyma-operator/operator/pkg/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type ComponentsAssociatedWithTemplate struct {
	ComponentName      string
	TemplateGeneration int64
	TemplateChannel    operatorv1alpha1.Channel
}

func SetComponentCRLabels(unstructuredCompCR *unstructured.Unstructured, componentName string, rel operatorv1alpha1.Channel) {
	labelMap := unstructuredCompCR.Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})
	labelMap[labels.ControllerName] = componentName
	labelMap[labels.Channel] = rel
	unstructuredCompCR.Object["metadata"].(map[string]interface{})["labels"] = labelMap
}

func GetGvkAndSpecFromTemplate(configMap *operatorv1alpha1.ModuleTemplate, componentName string) (*schema.GroupVersionKind, interface{}, error) {
	componentBytes, ok := configMap.Spec.Data[componentName]
	if !ok {
		return nil, nil, fmt.Errorf("%s component not found for resource in ConfigMap", componentName)
	}
	componentYaml, err := getTemplatedComponent(componentBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("error during config map template parsing %w", err)
	}

	return &schema.GroupVersionKind{
		Group:   componentYaml["group"].(string),
		Kind:    componentYaml["kind"].(string),
		Version: componentYaml["version"].(string),
	}, componentYaml["spec"], nil
}

func getTemplatedComponent(componentTemplate string) (map[string]interface{}, error) {
	componentYaml := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(componentTemplate), &componentYaml); err != nil {
		return nil, fmt.Errorf("error during config map unmarshal %w", err)
	}
	return componentYaml, nil
}

func AreTemplatesOutdated(logger *logr.Logger, k *operatorv1alpha1.Kyma, templates release.TemplatesByName) bool {
	for componentName, template := range templates {
		for _, condition := range k.Status.Conditions {
			if condition.Reason == componentName && template != nil {
				if template.GetGeneration() != condition.TemplateGeneration || operatorv1alpha1.Channel(template.Labels[labels.Channel]) != condition.TemplateChannel {
					logger.Info("detected outdated template",
						"condition", condition.Reason,
						"template", template.Name,
						"templateGeneration", template.GetGeneration(),
						"previousGeneration", condition.TemplateGeneration,
						"templateChannel", operatorv1alpha1.Channel(template.Labels[labels.Channel]),
						"previousChannel", condition.TemplateChannel,
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
