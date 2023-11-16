package parse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/channel"
	"github.com/kyma-project/lifecycle-manager/pkg/img"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
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
	EnableVerification  bool
	PublicKeyFilePath   string
}

func NewParser(
	clnt client.Client,
	inKCPMode bool,
	remoteSyncNamespace string,
	enableVerification bool,
	publicKeyFilePath string,
) *Parser {
	return &Parser{
		Client:              clnt,
		InKCPMode:           inKCPMode,
		remoteSyncNamespace: remoteSyncNamespace,
		EnableVerification:  enableVerification,
		PublicKeyFilePath:   publicKeyFilePath,
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
		descriptor, err := template.GetDescriptor()
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
		var manifest *v1beta2.Manifest
		if manifest, err = p.newManifestFromTemplate(ctx, module,
			template.ModuleTemplate); err != nil {
			template.Err = err
			modules = append(modules, &common.Module{
				ModuleName: module.Name,
				Template:   template,
			})
			continue
		}
		// we name the manifest after the module name
		manifest.SetName(name)
		// to have correct owner references, the manifest must always have the same namespace as kyma
		manifest.SetNamespace(kyma.GetNamespace())
		modules = append(modules, &common.Module{
			ModuleName: module.Name,
			FQDN:       fqdn,
			Version:    version,
			Template:   template,
			Manifest:   manifest,
		})
	}

	return modules
}

func overwriteNameAndNamespace(template *channel.ModuleTemplateTO, name, namespace string) {
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
		if template.Spec.Data != nil {
			manifest.Spec.Resource = template.Spec.Data.DeepCopy()
		}
	}

	clusterClient := p.Client
	if module.RemoteModuleTemplateRef != "" {
		syncContext, err := remote.SyncContextFromContext(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get syncContext: %w", err)
		}
		clusterClient = syncContext.RuntimeClient
	}

	var layers img.Layers
	var err error
	descriptor, err := template.GetDescriptor()
	if err != nil {
		return nil, fmt.Errorf("failed to get descriptor from template: %w", err)
	}
	verification, err := signature.NewVerification(ctx,
		clusterClient,
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

	if err := appendOptionalCustomStateCheck(manifest, template.Spec.CustomStateCheck); err != nil {
		return nil, fmt.Errorf("could not translate custom state check: %w", err)
	}

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
	manifest.Annotations[v1beta2.CustomStateCheckAnnotation] = string(stateCheckByte)
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
