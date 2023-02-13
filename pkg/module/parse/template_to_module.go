package parse

import (
	"errors"
	"fmt"

	ocm "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/pkg/channel"
	"github.com/kyma-project/lifecycle-manager/pkg/img"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/signature"
)

type ModuleConversionSettings struct {
	signature.Verification
}

var (
	ErrTemplateNotFound        = errors.New("template was not found")
	ErrUndefinedTargetToRemote = errors.New("target to remote relation undefined")
	ErrDefaultConfigParsing    = errors.New("defaultConfig could not be parsed")
)

func GenerateModulesFromTemplates(
	kyma *v1beta1.Kyma, templates channel.ModuleTemplatesByModuleName, verification signature.Verification,
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
	kyma *v1beta1.Kyma,
	templates channel.ModuleTemplatesByModuleName,
	settings *ModuleConversionSettings,
) (common.Modules, error) {
	// First, we fetch the module spec from the template and use it to resolve it into an arbitrary object
	// (since we do not know which module we are dealing with)
	modules := make(common.Modules, 0)

	for _, module := range kyma.Spec.Modules {
		template := templates[module.Name]
		if template == nil {
			return nil, fmt.Errorf("could not resolve template for module %s in %s: %w",
				module.Name, client.ObjectKeyFromObject(kyma), ErrTemplateNotFound,
			)
		}
		descriptor, err := template.Spec.GetUnsafeDescriptor()
		if err != nil {
			return nil, err
		}
		fqdn := descriptor.GetName()
		version := descriptor.GetVersion()
		name := common.CreateModuleName(fqdn, kyma.Name)
		// if the default data does not contain a name, default it to the module name
		if template.ModuleTemplate.Spec.Data.GetName() == "" {
			template.ModuleTemplate.Spec.Data.SetName(name)
		}
		// if the default data does not contain a namespace, default it to either the sync-namespace
		// or the kyma namespace
		if template.ModuleTemplate.Spec.Data.GetNamespace() == "" {
			if kyma.Spec.Sync.Namespace != "" {
				template.ModuleTemplate.Spec.Data.SetNamespace(kyma.Spec.Sync.Namespace)
			} else {
				template.ModuleTemplate.Spec.Data.SetNamespace(kyma.GetNamespace())
			}
		}
		var obj client.Object
		if obj, err = NewManifestFromTemplate(module, template.ModuleTemplate, settings.Verification); err != nil {
			return nil, err
		}
		// we name the manifest after the module name
		obj.SetName(name)
		// to have correct owner references, the manifest must always have the same namespace as kyma
		obj.SetNamespace(kyma.GetNamespace())
		modules = append(modules, &common.Module{
			ModuleName:       module.Name,
			FQDN:             fqdn,
			Version:          version,
			Template:         template.ModuleTemplate,
			TemplateOutdated: template.Outdated,
			Object:           obj,
		})
	}

	return modules, nil
}

func NewManifestFromTemplate(
	module v1beta1.Module,
	template *v1beta1.ModuleTemplate,
	verification signature.Verification,
) (*v1beta1.Manifest, error) {
	manifest := &v1beta1.Manifest{}
	manifest.Spec.Remote = ConvertTargetToRemote(template.Spec.Target)

	switch module.CustomResourcePolicy {
	case v1beta1.CustomResourcePolicyIgnore:
		manifest.Spec.Resource = nil
	case v1beta1.CustomResourcePolicyCreateAndDelete:
		fallthrough
	default:
		manifest.Spec.Resource = template.Spec.Data.DeepCopy()
	}

	var descriptor *ocm.ComponentDescriptor
	var layers img.Layers
	var err error

	if descriptor, err = template.Spec.GetUnsafeDescriptor(); err != nil {
		return nil, fmt.Errorf("could not decode the descriptor: %w", err)
	}

	if err := signature.Verify(descriptor, verification); err != nil {
		return nil, fmt.Errorf("could not verify descriptor: %w", err)
	}

	if layers, err = img.Parse(descriptor); err != nil {
		return nil, fmt.Errorf("could not parse descriptor: %w", err)
	}

	if err := translateLayersAndMergeIntoManifest(manifest, layers); err != nil {
		return nil, fmt.Errorf("could not translate layers and merge them: %w", err)
	}

	return manifest, nil
}

func translateLayersAndMergeIntoManifest(
	manifest *v1beta1.Manifest, layers img.Layers,
) error {
	for _, layer := range layers {
		if err := insertLayerIntoManifest(manifest, layer); err != nil {
			return fmt.Errorf("error in layer %s: %w", layer.LayerName, err)
		}
	}
	return nil
}

func insertLayerIntoManifest(
	manifest *v1beta1.Manifest, layer img.Layer,
) error {
	switch layer.LayerName {
	case img.CRDsLayer:
		fallthrough
	case img.ConfigLayer:
		ociImage, ok := layer.LayerRepresentation.(*img.OCI)
		if !ok {
			return fmt.Errorf("%w: not an OCIImage", ErrDefaultConfigParsing)
		}
		manifest.Spec.Config = v1beta1.ImageSpec{
			Repo:               ociImage.Repo,
			Name:               ociImage.Name,
			Ref:                ociImage.Ref,
			Type:               img.OCIRepresentationType,
			CredSecretSelector: ociImage.CredSecretSelector,
		}
	default:
		installRaw, err := layer.ToInstallRaw()
		if err != nil {
			return fmt.Errorf("error while merging the generic install representation: %w", err)
		}
		manifest.Spec.Install = v1beta1.InstallInfo{
			Source: runtime.RawExtension{Raw: installRaw},
			Name:   string(layer.LayerName),
		}
	}

	return nil
}

func ConvertTargetToRemote(remote v1beta1.Target) bool {
	switch remote {
	case v1beta1.TargetControlPlane:
		return false
	case v1beta1.TargetRemote:
		return true
	default:
		panic(ErrUndefinedTargetToRemote)
	}
}
