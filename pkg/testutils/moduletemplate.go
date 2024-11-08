package testutils

import (
	"context"
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

func GetModuleTemplate(ctx context.Context,
	clnt client.Client,
	module v1beta2.Module,
	defaultChannel string,
	namespace string,
) (*v1beta2.ModuleTemplate, error) {
	descriptorProvider := provider.NewCachedDescriptorProvider()
	templateLookup := templatelookup.NewTemplateLookup(clnt, descriptorProvider)
	availableModule := templatelookup.AvailableModule{
		Module: module,
	}
	templateInfo := templateLookup.PopulateModuleTemplateInfo(ctx, availableModule, namespace,
		defaultChannel)

	if templateInfo.Err != nil {
		return nil, fmt.Errorf("get module template: %w", templateInfo.Err)
	}
	return templateInfo.ModuleTemplate, nil
}

func ModuleTemplateExists(ctx context.Context,
	clnt client.Client,
	module v1beta2.Module,
	defaultChannel string,
	namespace string,
) error {
	moduleTemplate, err := GetModuleTemplate(ctx, clnt, module, defaultChannel, namespace)
	if moduleTemplate == nil || errors.Is(err, templatelookup.ErrNoTemplatesInListResult) {
		return ErrNotFound
	}

	return nil
}

func AllModuleTemplatesExists(ctx context.Context, clnt client.Client, kyma *v1beta2.Kyma) error {
	for _, module := range kyma.Spec.Modules {
		if err := ModuleTemplateExists(ctx, clnt, module, kyma.Spec.Channel, kyma.Namespace); err != nil {
			return err
		}
	}

	return nil
}

func UpdateModuleTemplateSpec(ctx context.Context,
	clnt client.Client,
	module v1beta2.Module,
	key,
	newValue,
	kymaChannel string,
	namespace string,
) error {
	moduleTemplate, err := GetModuleTemplate(ctx, clnt, module, kymaChannel, namespace)
	if err != nil {
		return err
	}
	if moduleTemplate.Spec.Data == nil {
		return ErrManifestResourceIsNil
	}
	moduleTemplate.Spec.Data.Object["spec"] = map[string]any{key: newValue}
	if err := clnt.Update(ctx, moduleTemplate); err != nil {
		return fmt.Errorf("update module tempate: %w", err)
	}
	return nil
}

func DeleteModuleTemplate(ctx context.Context,
	clnt client.Client, module v1beta2.Module, kymaChannel string, namespace string,
) error {
	moduleTemplate, err := GetModuleTemplate(ctx, clnt, module, kymaChannel, namespace)
	if util.IsNotFound(err) {
		return nil
	}

	err = client.IgnoreNotFound(clnt.Delete(ctx, moduleTemplate))
	if err != nil {
		return fmt.Errorf("moduletemplate not deleted: %w", err)
	}
	return nil
}

func ReadModuleVersionFromModuleTemplate(ctx context.Context, clnt client.Client, module v1beta2.Module,
	channel string, namespace string,
) (string, error) {
	moduleTemplate, err := GetModuleTemplate(ctx, clnt, module, channel, namespace)
	if err != nil {
		return "", fmt.Errorf("failed to fetch ModuleTemplate: %w", err)
	}

	descriptorProvider := provider.NewCachedDescriptorProvider()
	ocmDesc, err := descriptorProvider.GetDescriptor(moduleTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to get descriptor: %w", err)
	}

	return ocmDesc.Version, nil
}
