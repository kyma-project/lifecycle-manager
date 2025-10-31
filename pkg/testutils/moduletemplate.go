package testutils

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
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

func GetModuleTemplateInfo(ctx context.Context,
	clnt client.Client,
	module v1beta2.Module,
	kyma *v1beta2.Kyma,
) (*v1beta2.ModuleTemplate, *ocmidentity.ComponentId, error) {
	moduleTemplateInfoLookup := moduletemplateinfolookup.NewLookup(clnt)
	availableModule := templatelookup.ModuleInfo{
		Module: module,
	}

	moduleReleaseMeta, err := GetModuleReleaseMeta(ctx, module.Name, kyma.Namespace, clnt)
	if !meta.IsNoMatchError(err) && client.IgnoreNotFound(err) != nil {
		return nil, nil, fmt.Errorf("failed to get ModuleReleaseMeta: %w", err)
	}

	moduleTemplateInfo := moduleTemplateInfoLookup.Lookup(ctx, &availableModule, kyma, moduleReleaseMeta)

	if moduleTemplateInfo.Err != nil {
		return nil, nil, fmt.Errorf("failed to get module template: %w", moduleTemplateInfo.Err)
	}

	ocmIdentity, err := moduleTemplateInfo.GetOCMIdentity()
	return moduleTemplateInfo.ModuleTemplate, ocmIdentity, err
}

func ModuleTemplateExists(ctx context.Context,
	clnt client.Client,
	module v1beta2.Module,
	kyma *v1beta2.Kyma,
) error {
	moduleTemplate, _, err := GetModuleTemplateInfo(ctx, clnt, module, kyma)
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
	moduleTemplate, _, err := GetModuleTemplateInfo(ctx, clnt, module, kyma)
	if err != nil {
		return err
	}
	if moduleTemplate == nil {
		return fmt.Errorf("%w: moduleTemplate is nil", ErrNotFound)
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

// UpdateModuleTemplateWithFunc uses the provided function to update the ModuleTemplate resource.
// This function is intended to be used with "Eventually" assertions in tests.
// The provided updateFn should modify the ModuleTemplate resource in place and return an error
// if the modification fails.
// UpdateModuleTemplateWithFunc fetches the latest version of the ModuleTemplate resource before applying changes,
// to make sure the update is based on the most recent state.
func UpdateModuleTemplateWithFunc(ctx context.Context, clnt client.Client,
	mtName, mtNamespace string, updateFn func(mt *v1beta2.ModuleTemplate) error,
) error {
	moduleTemplate := &v1beta2.ModuleTemplate{}
	err := clnt.Get(ctx, client.ObjectKey{Name: mtName, Namespace: mtNamespace}, moduleTemplate)
	if err != nil {
		return fmt.Errorf("UpdateModuleTemplateWithFunc client.Get: %w", err)
	}
	err = updateFn(moduleTemplate)
	if err != nil {
		return err
	}
	err = clnt.Update(ctx, moduleTemplate)
	if err != nil {
		return fmt.Errorf("UpdateModuleTemplateWithFunc client.Update: %w", err)
	}
	return nil
}

func SetModuleTemplateBetaLabel(ctx context.Context, clnt client.Client, module v1beta2.Module,
	kyma *v1beta2.Kyma, betaValue bool,
) error {
	moduleTemplate, _, err := GetModuleTemplateInfo(ctx, clnt, module, kyma)
	if err != nil {
		return fmt.Errorf("failed to get module template: %w", err)
	}
	if moduleTemplate == nil {
		return fmt.Errorf("%w: moduleTemplate is nil", ErrNotFound)
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
	moduleTemplate, _, err := GetModuleTemplateInfo(ctx, clnt, module, kyma)
	if err != nil {
		return fmt.Errorf("failed to get module template: %w", err)
	}
	if moduleTemplate == nil {
		return fmt.Errorf("%w: moduleTemplate is nil", ErrNotFound)
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
		if moduleTemplateInfo.Spec.ModuleName == moduleName {
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
	moduleTemplate, _, err := GetModuleTemplateInfo(ctx, clnt, module, kyma)
	if util.IsNotFound(err) {
		return nil
	}

	err = client.IgnoreNotFound(clnt.Delete(ctx, moduleTemplate))
	if err != nil {
		return fmt.Errorf("moduletemplate not deleted: %w", err)
	}
	return nil
}

func GetOCMVersionForModule(ctx context.Context,
	clnt client.Client,
	module v1beta2.Module,
	kyma *v1beta2.Kyma,
) (string, error) {
	_, ocmIdentity, err := GetModuleTemplateInfo(ctx, clnt, module, kyma)
	if err != nil {
		return "", fmt.Errorf("failed to fetch ModuleTemplate: %w", err)
	}
	if ocmIdentity == nil {
		return "", fmt.Errorf("failed to get OCM identity: %w", ErrNotFound)
	}
	return ocmIdentity.Version(), nil
}
