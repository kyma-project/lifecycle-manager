package parse

import (
	"encoding/json"
	"errors"
	"fmt"

	ocm "github.com/gardener/component-spec/bindings-go/apis/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/channel"
	manifestV1alpha1 "github.com/kyma-project/module-manager/api/v1alpha1"
	"github.com/kyma-project/module-manager/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
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

	var manifest *manifestV1alpha1.Manifest

	for _, module := range kyma.Spec.Modules {
		template := templates[module.Name]
		if template == nil {
			return nil, fmt.Errorf("could not resolve template for module %s and resource %s: %w",
				module.Name, client.ObjectKeyFromObject(kyma), ErrTemplateNotFound,
			)
		}

		var err error

		template.ModuleTemplate.Spec.Data.SetName(common.CreateModuleName(module.Name, kyma.Name))
		template.ModuleTemplate.Spec.Data.SetNamespace(kyma.GetNamespace())

		if manifest, err = NewManifestFromTemplate(template.ModuleTemplate, settings.Verification); err != nil {
			return nil, err
		}
		modules[module.Name] = &common.Module{
			Name:             module.Name,
			Template:         template.ModuleTemplate,
			TemplateOutdated: template.Outdated,
			Manifest:         manifest,
		}
	}

	return modules, nil
}

func NewManifestFromTemplate(
	template *v1alpha1.ModuleTemplate,
	verification signature.Verification,
) (*manifestV1alpha1.Manifest, error) {
	manifest := &manifestV1alpha1.Manifest{}
	manifest.SetName(template.Spec.Data.GetName())
	manifest.SetNamespace(template.Spec.Data.GetNamespace())
	manifest.Spec.Remote = ConvertTargetToRemote(template.Spec.Target)
	template.Spec.Data.DeepCopyInto(&manifest.Spec.Resource)

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
	manifest *manifestV1alpha1.Manifest, layers img.Layers,
) error {
	for _, layer := range layers {
		if err := insertLayerIntoManifest(manifest, layer); err != nil {
			return fmt.Errorf("error in layer %s: %w", layer.LayerName, err)
		}
	}
	return nil
}

func insertLayerIntoManifest(
	manifest *manifestV1alpha1.Manifest, layer img.Layer,
) error {
	switch layer.LayerName {
	case img.CRDsLayer:
		fallthrough
	case img.ConfigLayer:
		ociImage, ok := layer.LayerRepresentation.(*img.OCI)
		if !ok {
			return fmt.Errorf("%w: not an OCIImage", ErrDefaultConfigParsing)
		}
		manifest.Spec.Config = types.ImageSpec{
			Repo:               ociImage.Repo,
			Name:               ociImage.Name,
			Ref:                ociImage.Ref,
			Type:               img.OCIRepresentationType,
			CredSecretSelector: ociImage.CredSecretSelector,
		}
	default:
		var installRaw []byte
		var err error

		if ociImage, ok := layer.LayerRepresentation.(*img.OCI); ok {
			installRaw, err = json.Marshal(ociImage)
			if err != nil {
				return fmt.Errorf("error while merging the oci install representation: %w", err)
			}
		}

		if helmImage, ok := layer.LayerRepresentation.(*img.Helm); ok {
			installRaw, err = json.Marshal(helmImage)
			if err != nil {
				return fmt.Errorf("error while merging the helm install representation: %w", err)
			}
		}
		manifest.Spec.Installs = append(
			manifest.Spec.Installs, manifestV1alpha1.InstallInfo{
				Source: runtime.RawExtension{Raw: installRaw},
				Name:   string(layer.LayerName),
			})
	}

	return nil
}

func ConvertTargetToRemote(remote v1alpha1.Target) bool {
	switch remote {
	case v1alpha1.TargetControlPlane:
		return false
	case v1alpha1.TargetRemote:
		return true
	default:
		panic(ErrUndefinedTargetToRemote)
	}
}
