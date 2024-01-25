package parse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"

	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/img"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
)

var ErrDefaultConfigParsing = errors.New("defaultConfig could not be parsed")

type Parser struct {
	client.Client
	descriptorProvider  *provider.CachedDescriptorProvider
	inKCPMode           bool
	remoteSyncNamespace string
}

func NewParser(clnt client.Client,
	descriptorProvider *provider.CachedDescriptorProvider,
	inKCPMode bool,
	remoteSyncNamespace string,
) *Parser {
	return &Parser{
		Client:              clnt,
		descriptorProvider:  descriptorProvider,
		inKCPMode:           inKCPMode,
		remoteSyncNamespace: remoteSyncNamespace,
	}
}

func (p *Parser) GenerateModulesFromTemplates(kyma *v1beta2.Kyma, templates templatelookup.ModuleTemplatesByModuleName,
) common.Modules {
	// First, we fetch the module spec from the template and use it to resolve it into an arbitrary object
	// (since we do not know which module we are dealing with)
	modules := make(common.Modules, 0)

	for _, module := range kyma.GetAvailableModules() {
		template := templates[module.Name]
		modules = p.appendModuleWithInformation(module, kyma, template, modules)
	}
	return modules
}

func (p *Parser) GenerateMandatoryModulesFromTemplates(ctx context.Context,
	kyma *v1beta2.Kyma,
	templates templatelookup.ModuleTemplatesByModuleName,
) common.Modules {
	modules := make(common.Modules, 0)

	for _, template := range templates {
		moduleName, ok := template.ObjectMeta.Labels[shared.ModuleName]
		if !ok {
			logf.FromContext(ctx).V(log.InfoLevel).Info("ModuleTemplate does not contain Module Name as label, "+
				"will fallback to use ModuleTemplate name as Module name",
				"template", template.Name)
			moduleName = template.Name
		}

		modules = p.appendModuleWithInformation(v1beta2.AvailableModule{
			Module: v1beta2.Module{
				Name:                 moduleName,
				CustomResourcePolicy: v1beta2.CustomResourcePolicyCreateAndDelete,
			},
			Enabled: true,
		}, kyma, template, modules)
	}

	return modules
}

func (p *Parser) appendModuleWithInformation(module v1beta2.AvailableModule, kyma *v1beta2.Kyma,
	template *templatelookup.ModuleTemplateInfo, modules common.Modules,
) common.Modules {
	if template.Err != nil && !errors.Is(template.Err, templatelookup.ErrTemplateNotAllowed) {
		modules = append(modules, &common.Module{
			ModuleName: module.Name,
			Template:   template,
			Enabled:    module.Enabled,
		})
		return modules
	}
	descriptor, err := p.descriptorProvider.GetDescriptor(template.ModuleTemplate)
	if err != nil {
		template.Err = err
		modules = append(modules, &common.Module{
			ModuleName: module.Name,
			Template:   template,
			Enabled:    module.Enabled,
		})
		return modules
	}
	fqdn := descriptor.GetName()
	name := common.CreateModuleName(fqdn, kyma.Name, module.Name)
	setNameAndNamespaceIfEmpty(template, name, p.remoteSyncNamespace)
	var manifest *v1beta2.Manifest
	if manifest, err = p.newManifestFromTemplate(module.Module,
		template.ModuleTemplate); err != nil {
		template.Err = err
		modules = append(modules, &common.Module{
			ModuleName: module.Name,
			Template:   template,
			Enabled:    module.Enabled,
		})
		return modules
	}
	// we name the manifest after the module name
	manifest.SetName(name)
	// to have correct owner references, the manifest must always have the same namespace as kyma
	manifest.SetNamespace(kyma.GetNamespace())
	modules = append(modules, &common.Module{
		ModuleName: module.Name,
		FQDN:       fqdn,
		Template:   template,
		Manifest:   manifest,
		Enabled:    module.Enabled,
	})
	return modules
}

func setNameAndNamespaceIfEmpty(template *templatelookup.ModuleTemplateInfo, name, namespace string) {
	if template.ModuleTemplate.Spec.Data == nil {
		return
	}
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
	module v1beta2.Module,
	template *v1beta2.ModuleTemplate,
) (*v1beta2.Manifest, error) {
	manifest := &v1beta2.Manifest{}
	manifest.Spec.Remote = p.inKCPMode

	switch module.CustomResourcePolicy {
	case v1beta2.CustomResourcePolicyIgnore:
		manifest.Spec.Resource = nil
	case v1beta2.CustomResourcePolicyCreateAndDelete:
		fallthrough
	default:
		if template.Spec.Data != nil {
			manifest.Spec.Resource = template.Spec.Data.DeepCopy()
		}
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

	if err := appendOptionalCustomStateCheck(manifest, template.Spec.CustomStateCheck); err != nil {
		return nil, fmt.Errorf("could not translate custom state check: %w", err)
	}
	manifest.Spec.Version = descriptor.Version
	return manifest, nil
}

func appendOptionalCustomStateCheck(manifest *v1beta2.Manifest, stateCheck []*v1beta2.CustomStateCheck) error {
	if manifest.Spec.Resource == nil || stateCheck == nil {
		return nil
	}
	if manifest.Annotations == nil {
		manifest.Annotations = make(map[string]string)
	}
	stateCheckByte, err := json.Marshal(stateCheck)
	if err != nil {
		return fmt.Errorf("failed to marshal stateCheck: %w", err)
	}
	manifest.Annotations[shared.CustomStateCheckAnnotation] = string(stateCheckByte)
	return nil
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
		manifest.Spec.Config = &v1beta2.ImageSpec{
			Repo:               ociImage.Repo,
			Name:               ociImage.Name,
			Ref:                ociImage.Ref,
			Type:               v1beta2.OciRefType,
			CredSecretSelector: ociImage.CredSecretSelector,
		}
	default:
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
