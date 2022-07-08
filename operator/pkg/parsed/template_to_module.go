package parsed

import (
	"errors"
	"fmt"

	ocm "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/gardener/component-spec/bindings-go/codec"
	"github.com/imdario/mergo"
	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/img"
	"github.com/kyma-project/kyma-operator/operator/pkg/release"
	"github.com/kyma-project/kyma-operator/operator/pkg/signature"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ModuleConversionSettings struct {
	signature.Verification
}

var ErrEmptyRawExtension = errors.New("raw extension is empty")

func Decode(ext runtime.RawExtension) (*ocm.ComponentDescriptor, error) {
	if len(ext.Raw) == 0 {
		return nil, ErrEmptyRawExtension
	}
	var descriptor ocm.ComponentDescriptor
	if err := codec.Decode(ext.Raw, &descriptor); err != nil {
		return nil, err
	}
	return &descriptor, nil
}

func TemplatesToModules(
	kyma *v1alpha1.Kyma,
	templates release.TemplatesInChannels,
	settings *ModuleConversionSettings,
) (Modules, error) {
	// First, we fetch the module spec from the template and use it to resolve it into an arbitrary object
	// (since we do not know which module we are dealing with)
	modules := make(Modules)

	var component *unstructured.Unstructured

	for _, module := range kyma.Spec.Modules {
		template := templates[module.Name]
		if template == nil {
			return nil, fmt.Errorf("could not find module %s for resource %s",
				module.Name, client.ObjectKeyFromObject(kyma))
		}

		var err error

		template.ModuleTemplate.Spec.Data.SetName(module.Name + kyma.Name)
		template.ModuleTemplate.Spec.Data.SetNamespace(kyma.GetNamespace())

		if component, err = NewModule(template.ModuleTemplate, settings.Verification); err != nil {
			return nil, err
		}
		modules[module.Name] = &Module{
			Template:         template.ModuleTemplate,
			TemplateOutdated: template.Outdated,
			Unstructured:     component,
			Settings:         module.Settings,
		}
	}

	return modules, nil
}

func NewModule(
	template *v1alpha1.ModuleTemplate,
	verification signature.Verification,
) (*unstructured.Unstructured, error) {
	component := &template.Spec.Data

	var descriptor *ocm.ComponentDescriptor
	var layers img.Layers
	var err error

	if descriptor, err = Decode(template.Spec.OCMDescriptor); err != nil {
		return nil, fmt.Errorf("could not decode the descriptor: %w", err)
	}

	if err := signature.Verify(descriptor, verification); err != nil {
		return nil, fmt.Errorf("could not verify descriptor: %w", err)
	}

	if layers, err = img.Parse(descriptor); err != nil {
		return nil, fmt.Errorf("could not parse descriptor: %w", err)
	}

	if err := translateLayersAndMergeIntoUnstructured(component, layers); err != nil {
		return nil, fmt.Errorf("could not translate layers and merge them: %w", err)
	}

	return component, nil
}

func translateLayersAndMergeIntoUnstructured(
	object *unstructured.Unstructured, layers img.Layers,
) error {
	if object.Object["spec"] == nil {
		object.Object["spec"] = make(map[string]any)
	}
	for _, layer := range layers {
		if err := translateLayerAndMergeIntoUnstructured(object, layer); err != nil {
			return fmt.Errorf("error in layer %s: %w", layer.LayerName, err)
		}
	}
	return nil
}

func translateLayerAndMergeIntoUnstructured(
	object *unstructured.Unstructured, layer img.Layer,
) error {
	var merge any
	install := map[string]any{
		"name": string(layer.LayerName),
		"type": string(layer.LayerType),
	}

	if err := mergo.Merge(&install, layer.LayerRepresentation.ToGenericRepresentation()); err != nil {
		return fmt.Errorf("error while merging the generic install representation: %w", err)
	}

	if layer.LayerName == "config" {
		r := layer.LayerRepresentation.(*img.OCIRef)
		merge = map[string]any{"config": map[string]any{
			"repo":   r.Repo,
			"module": r.Module,
			"ref":    r.Digest,
		}}
	} else {
		merge = map[string]any{"installs": []map[string]any{install}}
	}

	if err := mergo.Merge(&object.Object, map[string]any{"spec": merge}); err != nil {
		return fmt.Errorf("error while merging the layer representation into the spec: %w", err)
	}

	return nil
}
