package parser

import (
	"context"
	"errors"
	"fmt"

	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/extensions/accessmethods/ociartifact"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/img"
	modulecommon "github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
)

var ErrConvertingToOCIAccessSpec = errors.New("failed converting resource.AccessSpec to *ociartifact.AccessSpec")

type Parser struct {
	client.Client

	descriptorProvider  *provider.CachedDescriptorProvider
	remoteSyncNamespace string
}

func NewParser(clnt client.Client,
	descriptorProvider *provider.CachedDescriptorProvider,
	remoteSyncNamespace string,
) *Parser {
	return &Parser{
		Client:              clnt,
		descriptorProvider:  descriptorProvider,
		remoteSyncNamespace: remoteSyncNamespace,
	}
}

func (p *Parser) GenerateModulesFromTemplates(kyma *v1beta2.Kyma, templates templatelookup.ModuleTemplatesByModuleName,
) modulecommon.Modules {
	// First, we fetch the module spec from the template and use it to resolve it into an arbitrary object
	// (since we do not know which module we are dealing with)
	modules := make(modulecommon.Modules, 0)

	for _, module := range templatelookup.FetchModuleInfo(kyma) {
		template := templates[module.Name]
		modules = p.appendModuleWithInformation(module, kyma, template, modules)
	}
	return modules
}

func (p *Parser) GenerateMandatoryModulesFromTemplates(ctx context.Context,
	kyma *v1beta2.Kyma,
	templates templatelookup.ModuleTemplatesByModuleName,
) modulecommon.Modules {
	modules := make(modulecommon.Modules, 0)

	for _, template := range templates {
		modules = p.appendModuleWithInformation(templatelookup.ModuleInfo{
			Module: v1beta2.Module{
				Name:                 template.Name,
				CustomResourcePolicy: v1beta2.CustomResourcePolicyCreateAndDelete,
			},
			Enabled: true,
		}, kyma, template, modules)
	}

	return modules
}

func (p *Parser) appendModuleWithInformation(module templatelookup.ModuleInfo, kyma *v1beta2.Kyma,
	template *templatelookup.ModuleTemplateInfo, modules modulecommon.Modules,
) modulecommon.Modules {
	if template.Err != nil && !errors.Is(template.Err, templatelookup.ErrTemplateNotAllowed) {
		modules = append(modules, &modulecommon.Module{
			ModuleName:   module.Name,
			TemplateInfo: template,
			Enabled:      module.Enabled,
			IsUnmanaged:  module.Unmanaged,
		})
		return modules
	}
	descriptor, err := p.descriptorProvider.GetDescriptor(template.ModuleTemplate)
	if err != nil {
		template.Err = err
		modules = append(modules, &modulecommon.Module{
			ModuleName:   module.Name,
			TemplateInfo: template,
			Enabled:      module.Enabled,
			IsUnmanaged:  module.Unmanaged,
		})
		return modules
	}
	fqdn := descriptor.GetName()
	name := modulecommon.CreateModuleName(fqdn, kyma.Name, module.Name)
	setNameAndNamespaceIfEmpty(template, name, p.remoteSyncNamespace)
	var manifest *v1beta2.Manifest
	if manifest, err = p.newManifestFromTemplate(module.Module,
		template.ModuleTemplate); err != nil {
		template.Err = err
		modules = append(modules, &modulecommon.Module{
			ModuleName:   module.Name,
			TemplateInfo: template,
			Enabled:      module.Enabled,
			IsUnmanaged:  module.Unmanaged,
		})
		return modules
	}
	// we name the manifest after the module name
	manifest.SetName(name)
	// to have correct owner references, the manifest must always have the same namespace as kyma
	manifest.SetNamespace(kyma.GetNamespace())
	modules = append(modules, &modulecommon.Module{
		ModuleName:   module.Name,
		FQDN:         fqdn,
		TemplateInfo: template,
		Manifest:     manifest,
		Enabled:      module.Enabled,
		IsUnmanaged:  module.Unmanaged,
	})
	return modules
}

func setNameAndNamespaceIfEmpty(template *templatelookup.ModuleTemplateInfo, name, namespace string) {
	if template.Spec.Data == nil {
		return
	}
	// if the default data does not contain a name, default it to the module name
	if template.Spec.Data.GetName() == "" {
		template.Spec.Data.SetName(name)
	}
	// if the default data does not contain a namespace, default it to the provided namespace
	if template.Spec.Data.GetNamespace() == "" {
		template.Spec.Data.SetNamespace(namespace)
	}
}

func (p *Parser) newManifestFromTemplate(
	module v1beta2.Module,
	template *v1beta2.ModuleTemplate,
) (*v1beta2.Manifest, error) {
	manifest := &v1beta2.Manifest{}
	if manifest.Annotations == nil {
		manifest.Annotations = make(map[string]string)
	}

	manifest.Spec.CustomResourcePolicy = module.CustomResourcePolicy
	if template.Spec.Data != nil {
		manifest.Spec.Resource = template.Spec.Data.DeepCopy()
	}

	var layers img.Layers
	var err error
	descriptor, err := p.descriptorProvider.GetDescriptor(template)
	if err != nil {
		return nil, fmt.Errorf("failed to get descriptor from template: %w", err)
	}

	if layers, err = img.Parse(descriptor.ComponentDescriptor); err != nil {
		return nil, fmt.Errorf("could not parse descriptor: %w", err)
	}

	if err := translateLayersAndMergeIntoManifest(manifest, layers); err != nil {
		return nil, fmt.Errorf("could not translate layers and merge them: %w", err)
	}

	manifest.Spec.Version = descriptor.Version
	if localizedImages := getLocalizedImagesFromDescriptor(descriptor); len(localizedImages) > 0 {
		manifest.Spec.LocalizedImages = localizedImages
	}
	return manifest, nil
}

func getLocalizedImagesFromDescriptor(descriptor *types.Descriptor) []string {
	if descriptor == nil || descriptor.ComponentDescriptor == nil {
		return nil
	}
	localizedImages := make([]string, 0)
	for _, resource := range descriptor.Resources {
		access := resource.GetAccess()
		ocmAccessSpec, err := ocm.DefaultContext().AccessSpecForSpec(access)
		if err != nil {
			logf.Log.Error(fmt.Errorf("failed to create ocm spec for access: %w", err),
				"getLocalizedImagesFromDescriptor", "resourceName", resource.Name, "accessType", access.GetType())
			continue
		}

		if access.GetType() == ociartifact.Type {
			ociAccessSpec, ok := ocmAccessSpec.(*ociartifact.AccessSpec)
			if !ok {
				logf.Log.Error(fmt.Errorf("%w: actual type: %T", ErrConvertingToOCIAccessSpec, access),
					"getLocalizedImagesFromDescriptor")
				continue
			}
			if len(ociAccessSpec.ImageReference) > 0 {
				localizedImages = append(localizedImages, ociAccessSpec.ImageReference)
			}
		}
	}
	return localizedImages
}

func translateLayersAndMergeIntoManifest(manifest *v1beta2.Manifest, layers img.Layers) error {
	for _, layer := range layers {
		if err := insertLayerIntoManifest(manifest, layer); err != nil {
			return fmt.Errorf("error in layer %s: %w", layer.LayerName, err)
		}
	}
	return nil
}

func insertLayerIntoManifest(manifest *v1beta2.Manifest, layer img.Layer) error {
	switch layer.LayerName {
	case v1beta2.DefaultCRLayer:
		// default CR layer is not relevant for the manifest
	case v1beta2.ConfigLayer:
		imageSpec, err := layer.ConvertToImageSpec()
		if err != nil {
			return fmt.Errorf("error while parsing config layer: %w", err)
		}
		manifest.Spec.Config = imageSpec
	case v1beta2.RawManifestLayer:
		installRaw, err := layer.ToInstallRaw()
		if err != nil {
			return fmt.Errorf("error while merging the generic install representation: %w", err)
		}
		manifest.Spec.Install = v1beta2.InstallInfo{
			Source: machineryruntime.RawExtension{Raw: installRaw},
			Name:   string(layer.LayerName),
		}
	}

	return nil
}
