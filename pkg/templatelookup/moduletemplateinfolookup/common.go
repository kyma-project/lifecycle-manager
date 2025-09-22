package moduletemplateinfolookup

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/common"
)

func TemplateNameMatch(template *v1beta2.ModuleTemplate, name string) bool {
	if len(template.Spec.ModuleName) > 0 {
		return template.Spec.ModuleName == name
	}

	// Drop the legacyCondition once the label 'shared.ModuleName' is removed:
	// https://github.com/kyma-project/lifecycle-manager/issues/1796
	if template.Labels == nil {
		return false
	}
	return template.Labels[shared.ModuleName] == name
}

func newMoreThanOneTemplateCandidateErr(moduleName string,
	candidateTemplates []v1beta2.ModuleTemplate,
) error {
	candidates := make([]string, len(candidateTemplates))
	for i, candidate := range candidateTemplates {
		candidates[i] = candidate.GetName()
	}

	return fmt.Errorf("%w: more than one module template found for module: %s, candidates: %v",
		common.ErrTemplateNotIdentified, moduleName, candidates)
}

func getTemplateByVersion(ctx context.Context,
	clnt client.Reader,
	moduleName, moduleVersion, namespace string,
) (*v1beta2.ModuleTemplate, error) {
	moduleTemplate := &v1beta2.ModuleTemplate{}

	moduleTemplateName := fmt.Sprintf("%s-%s", moduleName, moduleVersion)
	if err := clnt.Get(ctx, client.ObjectKey{
		Name:      moduleTemplateName,
		Namespace: namespace,
	}, moduleTemplate); err != nil {
		return nil, fmt.Errorf("failed to get module template: %w", err)
	}

	return moduleTemplate, nil
}
