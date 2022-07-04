package util

import (
	"fmt"
	ocm "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/gardener/component-spec/bindings-go/codec"
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/img"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"github.com/kyma-project/kyma-operator/operator/pkg/release"
	errwrap "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Modules map[string]*Module
type Module struct {
	Name             string
	Template         *operatorv1alpha1.ModuleTemplate
	TemplateOutdated bool
	*unstructured.Unstructured
	Settings []operatorv1alpha1.Settings
}

func (m *Module) Channel() operatorv1alpha1.Channel {
	return m.Template.Spec.Channel
}

func SetComponentCRLabels(unstructuredCompCR *unstructured.Unstructured, componentName string, channel operatorv1alpha1.Channel, kymaName string) {
	labelMap := unstructuredCompCR.GetLabels()
	if labelMap == nil {
		labelMap = make(map[string]string)
	}
	labelMap[labels.ControllerName] = componentName
	labelMap[labels.Channel] = string(channel)
	labelMap[labels.KymaName] = kymaName
	unstructuredCompCR.SetLabels(labelMap)
}

func CopySettingsToUnstructuredFromResource(resource *unstructured.Unstructured, settings []operatorv1alpha1.Settings) {
	if len(settings) > 0 {
		resource.Object["spec"].(map[string]interface{})["customStates"] = settings
	}
}

func ParseTemplates(kyma *operatorv1alpha1.Kyma, templates release.TemplatesInChannels) (Modules, error) {
	// First, we fetch the component spec from the template and use it to resolve it into an arbitrary object
	// (since we do not know which component we are dealing with)
	modules := make(Modules)
	for _, component := range kyma.Spec.Components {
		template := templates[component.Name]
		if template == nil {
			return nil, fmt.Errorf("could not find template %s for resource %s",
				component.Name, client.ObjectKeyFromObject(kyma))
		}
		module, err := GetUnstructuredComponentFromTemplate(template, component.Name, kyma)
		if err != nil {
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

func GetUnstructuredComponentFromTemplate(template *release.TemplateInChannel, componentName string, kyma *operatorv1alpha1.Kyma) (*unstructured.Unstructured, error) {
	component := &unstructured.Unstructured{}
	if template.Template.Spec.Descriptor.String() != "" {
		var descriptor ocm.ComponentDescriptor
		if err := codec.Decode(template.Template.Spec.Descriptor.Raw, &descriptor); err != nil {
			return nil, errwrap.Wrap(err, "error while decoding the descriptor")
		}
		imgTemplate, err := img.ValidateAndParse(&descriptor, ".path")
		if err != nil {
			return nil, errwrap.Wrap(err, "error while parsing descriptor in module template")
		}

		for name, layer := range imgTemplate.Layers {
			if name == "config" {
				component.Object["spec"].(map[string]any)["config"] = layer
			}
			appendToSpecMap(component, "installs", map[string]any{
				"name": string(name),
				"chartRef": map[string]any{
					"repo":   layer.Repo,
					"module": layer.Module,
					"digest": layer.Digest,
				},
				"type": layer.LayerType,
			})
		}
	} else {
		component = &template.Template.Spec.Data
	}
	component.SetName(componentName + kyma.Name)
	component.SetNamespace(kyma.GetNamespace())
	return component, nil
}

func appendToSpecMap(unstruct *unstructured.Unstructured, key string, m ...map[string]any) {
	if unstruct.Object["spec"].(map[string]any)[key] == nil {
		unstruct.Object["spec"].(map[string]any)[key] = []any{m}
	} else {
		unstruct.Object["spec"].(map[string]any)[key] =
			append(unstruct.Object["spec"].(map[string]any)[key].([]any), m)
	}
}
