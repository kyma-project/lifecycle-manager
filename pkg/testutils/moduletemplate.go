package testutils

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/common"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

func CreateModuleTemplate(ctx context.Context,
	clnt client.Client,
	moduleTemplate *v1beta2.ModuleTemplate,
) error {
	moduleTemplate.SetResourceVersion("") // must be reset to enable retries
	if err := clnt.Create(ctx, moduleTemplate); client.IgnoreAlreadyExists(err) != nil {
		return fmt.Errorf("creating ModuleTemplate failed: %w", err)
	}
	return nil
}

func GetModuleTemplate(ctx context.Context,
	clnt client.Client,
	module v1beta2.Module,
	kyma *v1beta2.Kyma,
) (*v1beta2.ModuleTemplate, error) {
	moduleTemplateInfoLookupStrategies := moduletemplateinfolookup.NewModuleTemplateInfoLookupStrategies(
		[]moduletemplateinfolookup.ModuleTemplateInfoLookupStrategy{
			moduletemplateinfolookup.NewByVersionStrategy(clnt),
			moduletemplateinfolookup.NewByChannelStrategy(clnt),
			moduletemplateinfolookup.NewByModuleReleaseMetaStrategy(clnt),
		},
	)
	availableModule := templatelookup.ModuleInfo{
		Module: module,
	}

	moduleReleaseMeta, err := GetModuleReleaseMeta(ctx, module.Name, kyma.Namespace, clnt)
	if !meta.IsNoMatchError(err) && client.IgnoreNotFound(err) != nil {
		return nil, fmt.Errorf("failed to get ModuleReleaseMeta: %w", err)
	}

	templateInfo := moduleTemplateInfoLookupStrategies.Lookup(ctx, &availableModule, kyma, moduleReleaseMeta)

	if templateInfo.Err != nil {
		return nil, fmt.Errorf("get module template: %w", templateInfo.Err)
	}
	return templateInfo.ModuleTemplate, nil
}

func ModuleTemplateExists(ctx context.Context,
	clnt client.Client,
	module v1beta2.Module,
	kyma *v1beta2.Kyma,
) error {
	moduleTemplate, err := GetModuleTemplate(ctx, clnt, module, kyma)
	if moduleTemplate == nil || errors.Is(err, common.ErrNoTemplatesInListResult) {
		return ErrNotFound
	}

	return nil
}

func ModuleTemplateExistsByName(ctx context.Context,
	clnt client.Client,
	moduleName string,
	namespace string,
) error {
	if err := clnt.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      moduleName,
	}, &v1beta2.ModuleTemplate{}); err != nil {
		if util.IsNotFound(err) {
			return ErrNotFound
		}
	}
	return nil
}

func AllModuleTemplatesExists(ctx context.Context, clnt client.Client, kyma *v1beta2.Kyma) error {
	for _, module := range kyma.Spec.Modules {
		if err := ModuleTemplateExists(ctx, clnt, module, kyma); err != nil {
			return err
		}
	}

	return nil
}

func UpdateModuleTemplateSpec(ctx context.Context,
	clnt client.Client,
	module v1beta2.Module,
	key,
	newValue string,
	kyma *v1beta2.Kyma,
) error {
	moduleTemplate, err := GetModuleTemplate(ctx, clnt, module, kyma)
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

func SetModuleTemplateBetaLabel(ctx context.Context, clnt client.Client, module v1beta2.Module,
	kyma *v1beta2.Kyma, betaValue bool,
) error {
	moduleTemplate, err := GetModuleTemplate(ctx, clnt, module, kyma)
	if err != nil {
		return fmt.Errorf("failed to get module template: %w", err)
	}

	if moduleTemplate.Labels == nil {
		moduleTemplate.Labels = make(map[string]string)
	}

	if betaValue {
		moduleTemplate.Labels[shared.BetaLabel] = shared.EnableLabelValue
	} else {
		moduleTemplate.Labels[shared.BetaLabel] = shared.DisableLabelValue
	}

	if err := clnt.Update(ctx, moduleTemplate); err != nil {
		return fmt.Errorf("failed to update module template: %w", err)
	}

	return nil
}

func SetModuleTemplateInternalLabel(ctx context.Context, clnt client.Client, module v1beta2.Module,
	kyma *v1beta2.Kyma, internalValue bool,
) error {
	moduleTemplate, err := GetModuleTemplate(ctx, clnt, module, kyma)
	if err != nil {
		return fmt.Errorf("failed to get module template: %w", err)
	}

	if moduleTemplate.Labels == nil {
		moduleTemplate.Labels = make(map[string]string)
	}

	if internalValue {
		moduleTemplate.Labels[shared.InternalLabel] = shared.EnableLabelValue
	} else {
		moduleTemplate.Labels[shared.InternalLabel] = shared.DisableLabelValue
	}

	if err := clnt.Update(ctx, moduleTemplate); err != nil {
		return fmt.Errorf("failed to update module template: %w", err)
	}

	return nil
}

func MandatoryModuleTemplateHasExpectedLabel(ctx context.Context, clnt client.Client, moduleName, key, value string,
) error {
	mandatoryModuleTemplates, err := templatelookup.GetMandatory(ctx, clnt)
	if err != nil {
		return err
	}

	var moduleTemplate *v1beta2.ModuleTemplate
	for _, moduleTemplateInfo := range mandatoryModuleTemplates {
		if moduleTemplateInfo.Labels[shared.ModuleName] == moduleName {
			moduleTemplate = moduleTemplateInfo.ModuleTemplate
			break
		}
	}

	if moduleTemplate == nil {
		return fmt.Errorf("module template not found, %s", moduleName)
	}

	if moduleTemplate.Labels[key] != value {
		return fmt.Errorf("label %s:%s not found", key, value)
	}
	return nil
}

func DeleteModuleTemplate(ctx context.Context,
	clnt client.Client,
	module v1beta2.Module,
	kyma *v1beta2.Kyma,
) error {
	moduleTemplate, err := GetModuleTemplate(ctx, clnt, module, kyma)
	if util.IsNotFound(err) {
		return nil
	}

	err = client.IgnoreNotFound(clnt.Delete(ctx, moduleTemplate))
	if err != nil {
		return fmt.Errorf("moduletemplate not deleted: %w", err)
	}
	return nil
}

func ReadModuleVersionFromModuleTemplate(ctx context.Context,
	clnt client.Client,
	module v1beta2.Module,
	kyma *v1beta2.Kyma,
) (string, error) {
	moduleTemplate, err := GetModuleTemplate(ctx, clnt, module, kyma)
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
