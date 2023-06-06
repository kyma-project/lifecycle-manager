package parse

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/channel"
	"github.com/kyma-project/lifecycle-manager/pkg/img"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"
	"github.com/kyma-project/lifecycle-manager/pkg/signature"
)

type ModuleConversionSettings struct {
	signature.Verification
}

var ErrDefaultConfigParsing = errors.New("defaultConfig could not be parsed")

type Parser struct {
	client.Client
	InKCPMode           bool
	remoteSyncNamespace string
	*ocmextensions.ComponentDescriptorCache
	EnableVerification bool
	PublicKeyFilePath  string
}

func NewParser(
	clnt client.Client,
	descriptorCache *ocmextensions.ComponentDescriptorCache,
	inKCPMode bool,
	remoteSyncNamespace string,
	enableVerification bool,
	publicKeyFilePath string,
) *Parser {
	return &Parser{
		Client:                   clnt,
		ComponentDescriptorCache: descriptorCache,
		InKCPMode:                inKCPMode,
		remoteSyncNamespace:      remoteSyncNamespace,
		EnableVerification:       enableVerification,
		PublicKeyFilePath:        publicKeyFilePath,
	}
}

func (p *Parser) GenerateModulesFromTemplates(ctx context.Context,
	kyma *v1beta2.Kyma,
	templates channel.ModuleTemplatesByModuleName,
) common.Modules {
	// First, we fetch the module spec from the template and use it to resolve it into an arbitrary object
	// (since we do not know which module we are dealing with)
	modules := make(common.Modules, 0)

	for _, module := range kyma.Spec.Modules {
		template := templates[module.Name]
		if template.Err != nil && !errors.Is(template.Err, channel.ErrTemplateNotAllowed) {
			modules = append(modules, &common.Module{
				ModuleName: module.Name,
				Template:   template,
			})
			continue
		}
		descriptor, err := template.Spec.GetDescriptor()
		if err != nil {
			template.Err = err
			modules = append(modules, &common.Module{
				ModuleName: module.Name,
				Template:   template,
			})
			continue
		}
		fqdn := descriptor.GetName()
		version := descriptor.GetVersion()
		name := common.CreateModuleName(fqdn, kyma.Name, module.Name)
		overwriteNameAndNamespace(template, name, p.remoteSyncNamespace)
		var obj client.Object
		if obj, err = p.newManifestFromTemplate(ctx, module,
			template.ModuleTemplate); err != nil {
			template.Err = err
			modules = append(modules, &common.Module{
				ModuleName: module.Name,
				Template:   template,
			})
			continue
		}
		// we name the manifest after the module name
		obj.SetName(name)
		// to have correct owner references, the manifest must always have the same namespace as kyma
		obj.SetNamespace(kyma.GetNamespace())
		modules = append(modules, &common.Module{
			ModuleName: module.Name,
			FQDN:       fqdn,
			Version:    version,
			Template:   template,
			Object:     obj,
		})
	}

	return modules
}

func overwriteNameAndNamespace(template *channel.ModuleTemplateTO, name, namespace string) {
	// if the default data does not contain a name, default it to the module name
	if template.ModuleTemplate.Spec.Data.GetName() == "" {
		template.ModuleTemplate.Spec.Data.SetName(name)
	}
	// if the default data does not contain a namespace, default it to the provided namespace
	if template.ModuleTemplate.Spec.Data.GetNamespace() == "" {
		template.ModuleTemplate.Spec.Data.SetNamespace(namespace)
	}
}

func (p *Parser) newManifestFromTemplate(
	ctx context.Context,
	module v1beta2.Module,
	template *v1beta2.ModuleTemplate,
) (*v1beta2.Manifest, error) {
	manifest := &v1beta2.Manifest{}
	manifest.Spec.Remote = p.InKCPMode

	switch module.CustomResourcePolicy {
	case v1beta2.CustomResourcePolicyIgnore:
		manifest.Spec.Resource = nil
	case v1beta2.CustomResourcePolicyCreateAndDelete:
		fallthrough
	default:
		manifest.Spec.Resource = template.Spec.Data.DeepCopy()
	}

	var layers img.Layers
	var err error
	descriptor, err := template.Spec.GetDescriptor()
	if err != nil {
		return nil, err
	}
	verification, err := signature.NewVerification(ctx,
		p.Client,
		p.EnableVerification,
		p.PublicKeyFilePath,
		module.Name)
	if err != nil {
		return nil, err
	}

	if err := signature.Verify(descriptor.ComponentDescriptor, verification); err != nil {
		return nil, fmt.Errorf("could not verify signature: %w", err)
	}

	if layers, err = img.Parse(descriptor.ComponentDescriptor); err != nil {
		return nil, fmt.Errorf("could not parse descriptor: %w", err)
	}

	if err := translateLayersAndMergeIntoManifest(manifest, layers); err != nil {
		return nil, fmt.Errorf("could not translate layers and merge them: %w", err)
	}

	return manifest, nil
}

func translateLayersAndMergeIntoManifest(
	manifest *v1beta2.Manifest, layers img.Layers,
) error {
	for _, layer := range layers {
		if err := insertLayerIntoManifest(manifest, layer); err != nil {
			return fmt.Errorf("error in layer %s: %w", layer.LayerName, err)
		}
	}
	return nil
}

func insertLayerIntoManifest(
	manifest *v1beta2.Manifest, layer img.Layer,
) error {
	switch layer.LayerName {
	case img.CRDsLayer:
		fallthrough
	case img.ConfigLayer:
		ociImage, ok := layer.LayerRepresentation.(*img.OCI)
		if !ok {
			return fmt.Errorf("%w: not an OCIImage", ErrDefaultConfigParsing)
		}
		manifest.Spec.Config = v1beta2.ImageSpec{
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
		manifest.Spec.Install = v1beta2.InstallInfo{
			Source: runtime.RawExtension{Raw: installRaw},
			Name:   string(layer.LayerName),
		}
	}

	return nil
}
