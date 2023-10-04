package testutils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"
	compdesc2 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ModuleTemplateFactory(
	module v1beta2.Module,
	data unstructured.Unstructured,
	onPrivateRepo bool,
	isInternal bool,
	isBeta bool,
	isClusterScoped bool,
) (*v1beta2.ModuleTemplate, error) {
	template, err := ModuleTemplateFactoryForSchema(module, data, compdesc2.SchemaVersion, onPrivateRepo)
	if err != nil {
		return nil, err
	}
	if isInternal {
		template.Labels[v1beta2.InternalLabel] = v1beta2.EnableLabelValue
	}
	if isBeta {
		template.Labels[v1beta2.BetaLabel] = v1beta2.EnableLabelValue
	}
	if isClusterScoped {
		if template.Annotations == nil {
			template.Annotations = make(map[string]string)
		}
		template.Annotations[v1beta2.IsClusterScopedAnnotation] = v1beta2.EnableLabelValue
	}
	return template, nil
}

func ModuleTemplateFactoryForSchema(
	module v1beta2.Module,
	data unstructured.Unstructured,
	schemaVersion compdesc.SchemaVersion,
	onPrivateRepo bool,
) (*v1beta2.ModuleTemplate, error) {
	var moduleTemplate v1beta2.ModuleTemplate
	var err error
	switch schemaVersion {
	case compdesc2.SchemaVersion:
		err = readModuleTemplateWithV2Schema(&moduleTemplate)
	case v3alpha1.GroupVersion:
		fallthrough
	case v3alpha1.SchemaVersion:
		fallthrough
	default:
		err = readModuleTemplateWithV3Schema(&moduleTemplate)
	}
	if onPrivateRepo {
		err = readModuleTemplateWithinPrivateRepo(&moduleTemplate)
	}
	if err != nil {
		return &moduleTemplate, err
	}
	moduleTemplate.Name = module.Name
	if moduleTemplate.Labels == nil {
		moduleTemplate.Labels = make(map[string]string)
	}
	moduleTemplate.Labels[v1beta2.ModuleName] = module.Name
	moduleTemplate.Spec.Channel = module.Channel
	if data.GetKind() != "" {
		moduleTemplate.Spec.Data = &data
	}
	return &moduleTemplate, nil
}

func readModuleTemplateWithV2Schema(moduleTemplate *v1beta2.ModuleTemplate) error {
	template := "operator_v1beta2_moduletemplate_kcp-module.yaml"
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		panic("Can't capture current filename!")
	}

	modulePath := filepath.Join(filepath.Dir(filename),
		"../../config/samples/component-integration-installed", template)

	moduleFile, err := os.ReadFile(modulePath)
	if err != nil {
		return fmt.Errorf("read module template: %w", err)
	}
	err = yaml.Unmarshal(moduleFile, &moduleTemplate)
	if err != nil {
		return fmt.Errorf("unmarshal module template: %w", err)
	}
	return nil
}

func readModuleTemplateWithinPrivateRepo(moduleTemplate *v1beta2.ModuleTemplate) error {
	template := "operator_v1beta2_moduletemplate_kcp-module-cred-label.yaml"
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		panic("Can't capture current filename!")
	}
	modulePath := filepath.Join(
		filepath.Dir(filename), "../../config/samples/component-integration-installed", template,
	)

	moduleFile, err := os.ReadFile(modulePath)
	if err != nil {
		return fmt.Errorf("read module template: %w", err)
	}
	if err := yaml.Unmarshal(moduleFile, &moduleTemplate); err != nil {
		return fmt.Errorf("unmarshal module template: %w", err)
	}
	return nil
}

func readModuleTemplateWithV3Schema(moduleTemplate *v1beta2.ModuleTemplate) error {
	template := "operator_v1beta2_moduletemplate_ocm.software.v3alpha1.yaml"
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		panic("Can't capture current filename!")
	}
	modulePath := filepath.Join(
		filepath.Dir(filename), "../../config/samples/component-integration-installed", template,
	)

	moduleFile, err := os.ReadFile(modulePath)
	if err != nil {
		return fmt.Errorf("read module template: %w", err)
	}
	if err := yaml.Unmarshal(moduleFile, &moduleTemplate); err != nil {
		return fmt.Errorf("unmarshal module template: %w", err)
	}
	return nil
}

func DeployModuleTemplate(
	ctx context.Context,
	kcpClient client.Client,
	module v1beta2.Module,
	onPrivateRepo,
	isInternal,
	isBeta bool,
	isClusterScoped bool,
) error {
	template, err := ModuleTemplateFactory(module, unstructured.Unstructured{}, onPrivateRepo, isInternal, isBeta,
		isClusterScoped)
	if err != nil {
		return err
	}

	if err := kcpClient.Create(ctx, template); err != nil {
		return fmt.Errorf("create module template: %w", err)
	}
	return nil
}

func GetModuleTemplate(ctx context.Context,
	clnt client.Client, name, namespace string,
) (*v1beta2.ModuleTemplate, error) {
	moduleTemplateInCluster := &v1beta2.ModuleTemplate{}
	moduleTemplateInCluster.SetNamespace(namespace)
	moduleTemplateInCluster.SetName(name)
	err := clnt.Get(ctx, client.ObjectKeyFromObject(moduleTemplateInCluster), moduleTemplateInCluster)
	if err != nil {
		return nil, fmt.Errorf("get module template: %w", err)
	}
	return moduleTemplateInCluster, nil
}

func ModuleTemplateExists(ctx context.Context, client client.Client, name, namespace string) error {
	moduleTemplate, err := GetModuleTemplate(ctx, client, name, namespace)
	return CRExists(moduleTemplate, err)
}

func AllModuleTemplatesExists(ctx context.Context,
	clnt client.Client, kyma *v1beta2.Kyma, remoteSyncNamespace string,
) error {
	for _, module := range kyma.Spec.Modules {
		if err := ModuleTemplateExists(ctx, clnt, module.Name, remoteSyncNamespace); err != nil {
			return err
		}
	}

	return nil
}
