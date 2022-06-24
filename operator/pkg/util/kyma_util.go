package util

import (
	"fmt"
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"github.com/kyma-project/kyma-operator/operator/pkg/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ComponentsAssociatedWithTemplate struct {
	ComponentName      string
	TemplateGeneration int64
	TemplateChannel    operatorv1alpha1.Channel
}

func SetComponentCRLabels(unstructuredCompCR *unstructured.Unstructured, componentName string, channel operatorv1alpha1.Channel, kymaName string) {
	labelMap := unstructuredCompCR.GetLabels()
	if labelMap == nil {
		labelMap = make(map[string]string)
	}
	labelMap[labels.ControllerName] = componentName
	labelMap[labels.Channel] = string(channel)
	labelMap[labels.ComponentOwner] = kymaName
	unstructuredCompCR.SetLabels(labelMap)
}

func CopyComponentSettingsToUnstructuredFromResource(resource *unstructured.Unstructured, component operatorv1alpha1.ComponentType) {
	if len(component.CustomStates) > 0 {
		resource.Object["spec"].(map[string]interface{})["customStates"] = component.CustomStates
	}
}

func GetUnstructuredComponentFromTemplate(templates release.TemplateLookupResultsByName, componentName string, kyma *operatorv1alpha1.Kyma) (*unstructured.Unstructured, error) {
	lookupResult := templates[componentName]
	if lookupResult == nil {
		return nil, fmt.Errorf("could not find template %s for resource %s",
			componentName, client.ObjectKeyFromObject(kyma))
	}

	desiredComponentStruct := &lookupResult.Template.Spec.Data
	desiredComponentStruct.SetName(componentName + kyma.Name)
	desiredComponentStruct.SetNamespace(kyma.GetNamespace())

	return desiredComponentStruct.DeepCopy(), nil
}
