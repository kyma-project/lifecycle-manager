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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ModuleConversionSettings struct {
	signature.Verification
}

var (
	ErrEmptyRawExtension    = errors.New("raw extension is empty")
	ErrDefaultConfigParsing = errors.New("defaultConfig could not be parsed")
)

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

func ResolveModuleCRName(kyma *v1alpha1.Kyma, name string) string {
	return name + kyma.Name
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
		module := module
		template := templates[module.Name]
		if template == nil {
			return nil, fmt.Errorf("could not find module %s for resource %s",
				module.Name, client.ObjectKeyFromObject(kyma))
		}

		var err error
		if component, err = NewModule(template.ModuleTemplate, settings.Verification); err != nil {
			return nil, err
		}
		component.SetName(ResolveModuleCRName(kyma, module.Name))
		component.SetNamespace(kyma.GetNamespace())

		modules[module.Name] = &Module{
			Name:             module.Name,
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

	if !template.Spec.Remote {
		return component, nil
	}

	manifest := &unstructured.Unstructured{}
	manifest.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "component.kyma-project.io",
		Version: "v1alpha1",
		Kind:    "Manifest",
	})
	manifest.Object["spec"] = map[string]any{"data": component}

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

	if err := translateLayersAndMergeIntoUnstructured(manifest, layers); err != nil {
		return nil, fmt.Errorf("could not translate layers and merge them: %w", err)
	}

	return manifest, nil
}

func translateLayersAndMergeIntoUnstructured(
	object *unstructured.Unstructured, layers img.Layers,
) error {
	if object.Object["spec"] == nil {
		object.Object["spec"] = make(map[string]any)
	}
	for _, layer := range layers {
		if err := translateLayerIntoInstall(object, layer); err != nil {
			return fmt.Errorf("error in layer %s: %w", layer.LayerName, err)
		}
	}
	return nil
}

func translateLayerIntoInstall(
	component *unstructured.Unstructured, layer img.Layer,
) error {
	var merge any
	install := map[string]any{"name": string(layer.LayerName)}
	source := map[string]any{"source": layer.LayerRepresentation.ToGenericRepresentation()}

	if err := mergo.Merge(&install, &source); err != nil {
		return fmt.Errorf("error while merging the generic install representation: %w", err)
	}

	if layer.LayerName == "config" {
		ociImage, ok := layer.LayerRepresentation.(*img.OCI)
		if !ok {
			return fmt.Errorf("%w: not an OCIImage", ErrDefaultConfigParsing)
		}
		merge = map[string]any{"defaultConfig": ociImage.ToGenericRepresentation()}
	} else {
		merge = map[string]any{"installs": []map[string]any{install}}
	}
	if err := mergo.Merge(&component.Object, map[string]any{"spec": merge}, mergo.WithAppendSlice); err != nil {
		return fmt.Errorf("error while merging the layer representation into the spec: %w", err)
	}

	return nil
}
