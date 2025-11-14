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

var (
	ErrConvertingToOCIAccessSpec = errors.New("failed converting resource.AccessSpec to *ociartifact.AccessSpec")
	ErrConvertingToImgOCI        = errors.New("failed converting layerRepresentation to *img.OCI")
)

type Parser struct {
	client.Client

	descriptorProvider  *provider.CachedDescriptorProvider
	remoteSyncNamespace string
	ociRepo             string
}

func NewParser(clnt client.Client,
	descriptorProvider *provider.CachedDescriptorProvider,
	remoteSyncNamespace string,
	ociRepo string,
) *Parser {
	return &Parser{
		Client:              clnt,
		descriptorProvider:  descriptorProvider,
		remoteSyncNamespace: remoteSyncNamespace,
		ociRepo:             ociRepo,
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
				Name:                 template.Spec.ModuleName,
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
	descriptor, err := p.descriptorProvider.GetDescriptorWithIdentity(template)
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
	if manifest, err = newManifestFromTemplate(module.Module,
		template.ModuleTemplate, descriptor, p.ociRepo); err != nil {
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

func newManifestFromTemplate(
	module v1beta2.Module,
	template *v1beta2.ModuleTemplate,
	descriptor *types.Descriptor,
	repo string,
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

	if layers, err = img.Parse(descriptor.ComponentDescriptor); err != nil {
		return nil, fmt.Errorf("could not parse descriptor: %w", err)
	}

	if err := translateLayersAndMergeIntoManifest(manifest, layers, repo); err != nil {
		return nil, fmt.Errorf("could not translate layers and merge them: %w", err)
	}

	manifest.Spec.Version = descriptor.Version
	if localizedImages := getLocalizedImagesFromDescriptor(descriptor); len(localizedImages) > 0 {
		manifest.Spec.LocalizedImages = localizedImages
	}
	if template.Spec.Manager != nil {
		manifest.Spec.Manager = template.Spec.Manager.DeepCopy()
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

func translateLayersAndMergeIntoManifest(manifest *v1beta2.Manifest, layers img.Layers, repo string) error {
	for _, layer := range layers {
		if err := insertLayerIntoManifest(manifest, layer, repo); err != nil {
			return fmt.Errorf("error in layer %s: %w", layer.LayerName, err)
		}
	}
	return nil
}

func insertLayerIntoManifest(manifest *v1beta2.Manifest, layer img.Layer, ociRepoFromConfig string) error {
	switch layer.LayerName {
	case v1beta2.DefaultCRLayer:
		// default CR layer is not relevant for the manifest
	case v1beta2.ConfigLayer:
		imageSpec, err := layer.ConvertToImageSpec(ociRepoFromConfig)
		if err != nil {
			return fmt.Errorf("error while parsing config layer: %w", err)
		}
		manifest.Spec.Config = imageSpec
	case v1beta2.RawManifestLayer:
		ociImage, ok := layer.LayerRepresentation.(*img.OCI)
		if !ok {
			return fmt.Errorf("%w: actual type: %T", ErrConvertingToImgOCI, layer.LayerRepresentation)
		}

		// For fetching data from the OCI registry use the repo from the global config
		// instead of the one from the layer (it is the same as in the ComponentDescriptor).
		// These two values may be different and the explicitly configured one is safer to use,
		// as it is known to be reachable.
		// After all, we've been able to read the ComponentDescriptor using it.
		ociImageCopy := img.OCI{
			Repo: ociRepoFromConfig,
			Name: ociImage.Name,
			Ref:  ociImage.Ref,
			Type: ociImage.Type,
		}

		installRaw, err := ociImageCopy.ToInstallRaw()
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
