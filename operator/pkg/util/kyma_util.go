package util

import (
	"fmt"

	"github.com/imdario/mergo"
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"github.com/kyma-project/kyma-operator/operator/pkg/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Modules map[string]*Module
type Module struct {
	Name             string
	Template         *operatorv1alpha1.ModuleTemplate
	TemplateOutdated bool
	*unstructured.Unstructured
	Settings unstructured.Unstructured
}

func (m *Module) Channel() operatorv1alpha1.Channel {
	return m.Template.Spec.Channel
}

func SetComponentCRLabels(unstructuredCompCR *unstructured.Unstructured, componentName string,
	channel operatorv1alpha1.Channel, kymaName string,
) {
	labelMap := unstructuredCompCR.GetLabels()
	if labelMap == nil {
		labelMap = make(map[string]string)
	}

	labelMap[labels.ControllerName] = componentName
	labelMap[labels.Channel] = string(channel)
	labelMap[labels.KymaName] = kymaName
	unstructuredCompCR.SetLabels(labelMap)
}

func CopySettingsToUnstructuredFromResource(resource *unstructured.Unstructured, settings unstructured.Unstructured) error {
	overrideSpec := settings.Object["spec"]

	if overrideSpec != nil {
		if err := mergo.Merge(resource.Object["spec"], overrideSpec); err != nil {
			return err
		}
	}
	return nil
}

func ParseTemplates(kyma *operatorv1alpha1.Kyma, templates release.TemplatesInChannels) (Modules, error) {
	// First, we fetch the component spec from the template and use it to resolve it into an arbitrary object
	// (since we do not know which component we are dealing with)
	modules := make(Modules)

	var module *unstructured.Unstructured

	for _, component := range kyma.Spec.Components {
		template := templates[component.Name]
		if template == nil {
			return nil, fmt.Errorf("could not find template %s for resource %s",
				component.Name, client.ObjectKeyFromObject(kyma))
		}

		var err error
		if module, err = GetUnstructuredComponentFromTemplate(template, component.Name, kyma); err != nil {
			return nil, err
		}

		modules[component.Name] = &Module{
			Template:         template.Template,
			TemplateOutdated: template.Outdated,
			Unstructured:     module,
			Settings:         component.Settings,
		}
	}

	return modules, nil
}

func GetUnstructuredComponentFromTemplate(template *release.TemplateInChannel, componentName string,
	kyma *operatorv1alpha1.Kyma,
) (*unstructured.Unstructured, error) {
	desiredComponentStruct := &template.Template.Spec.Data
	desiredComponentStruct.SetName(componentName + kyma.Name)
	desiredComponentStruct.SetNamespace(kyma.GetNamespace())

	return desiredComponentStruct.DeepCopy(), nil
}
