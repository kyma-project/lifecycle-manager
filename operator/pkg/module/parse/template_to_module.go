package parse

import (
	"errors"
	"fmt"

	ocm "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/imdario/mergo"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/channel"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/img"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/signature"
)

type ModuleConversionSettings struct {
	signature.Verification
}

var (
	ErrTemplateNotFound     = errors.New("template was not found")
	ErrEmptyRawExtension    = errors.New("raw extension is empty")
	ErrDefaultConfigParsing = errors.New("defaultConfig could not be parsed")
)

func GenerateModulesFromTemplates(
	kyma *v1alpha1.Kyma, templates channel.ModuleTemplatesByModuleName, verification signature.Verification,
) (common.Modules, error) {
	// these are the actual modules
	modules, err := templatesToModules(kyma, templates,
		&ModuleConversionSettings{Verification: verification})
	if err != nil {
		return nil, fmt.Errorf("cannot convert templates: %w", err)
	}

	return modules, nil
}

func templatesToModules(
	kyma *v1alpha1.Kyma,
	templates channel.ModuleTemplatesByModuleName,
	settings *ModuleConversionSettings,
) (common.Modules, error) {
	// First, we fetch the module spec from the template and use it to resolve it into an arbitrary object
	// (since we do not know which module we are dealing with)
	modules := make(common.Modules)

	var component *unstructured.Unstructured

	for _, module := range kyma.Spec.Modules {
		template := templates[module.Name]
		if template == nil {
			return nil, fmt.Errorf("could not resolve template for module %s and resource %s: %w",
				module.Name, client.ObjectKeyFromObject(kyma), ErrTemplateNotFound)
		}

		var err error

		template.ModuleTemplate.Spec.Data.SetName(common.CreateModuleName(module.Name, kyma.Name))
		template.ModuleTemplate.Spec.Data.SetNamespace(kyma.GetNamespace())

		if component, err = NewModule(template.ModuleTemplate, settings.Verification); err != nil {
			return nil, err
		}
		modules[module.Name] = &common.Module{
			Name:             module.Name,
			Template:         template.ModuleTemplate,
			TemplateOutdated: template.Outdated,
			Unstructured:     component,
		}
	}

	return modules, nil
}

func NewModule(
	template *v1alpha1.ModuleTemplate,
	verification signature.Verification,
) (*unstructured.Unstructured, error) {
	component := template.Spec.Data.DeepCopy()
	resource := template.Spec.Data.DeepCopy()
	if err := mergeResourceIntoSpec(resource, component); err != nil {
		return nil, err
	}
	if err := mergeTargetIntoSpec(template.Spec.Target, component); err != nil {
		return nil, err
	}
	component.SetKind("Manifest")
	var descriptor *ocm.ComponentDescriptor
	var layers img.Layers
	var err error

	if descriptor, err = template.Spec.GetDescriptor(); err != nil {
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
	for _, layer := range layers {
		if err := translateLayerIntoSpec(object, layer); err != nil {
			return fmt.Errorf("error in layer %s: %w", layer.LayerName, err)
		}
	}
	return nil
}

func translateLayerIntoSpec(
	component *unstructured.Unstructured, layer img.Layer,
) error {
	var merge any
	var err error
	if layer.LayerName == img.CRDsLayer || layer.LayerName == img.ConfigLayer {
		ociImage, ok := layer.LayerRepresentation.(*img.OCI)
		if !ok {
			return fmt.Errorf("%w: not an OCIImage", ErrDefaultConfigParsing)
		}
		merge = map[string]any{string(layer.LayerName): ociImage.ToGenericRepresentation()}
	} else {
		if merge, err = mergeInstalls(layer); err != nil {
			return err
		}
	}
	if err := mergo.Merge(&component.Object, map[string]any{"spec": merge}, mergo.WithAppendSlice); err != nil {
		return fmt.Errorf("error while merging the layer representation into the spec: %w", err)
	}

	return nil
}

func mergeInstalls(layer img.Layer) (any, error) {
	install := map[string]any{"name": string(layer.LayerName)}
	source := map[string]any{"source": layer.LayerRepresentation.ToGenericRepresentation()}

	if err := mergo.Merge(&install, &source); err != nil {
		return nil, fmt.Errorf("error while merging the generic install representation: %w", err)
	}
	merge := map[string]any{string(img.InstallLayer): []map[string]any{install}}
	return merge, nil
}
