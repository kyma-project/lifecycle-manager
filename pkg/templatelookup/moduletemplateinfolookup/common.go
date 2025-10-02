package moduletemplateinfolookup

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func TemplateNameMatch(template *v1beta2.ModuleTemplate, name string) bool {
	return template.Spec.ModuleName == name
}

func getTemplateByVersion(ctx context.Context,
	clnt client.Reader,
	moduleName,
	moduleVersion,
	namespace string,
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
