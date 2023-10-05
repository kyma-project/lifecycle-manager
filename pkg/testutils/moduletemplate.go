package testutils

import (
	"context"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/channel"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetModuleTemplate(ctx context.Context,
	clnt client.Client,
	module v1beta2.Module,
	defaultChannel string,
) (*v1beta2.ModuleTemplate, error) {
	templateTO := channel.NewTemplateLookup(clnt, module, defaultChannel).WithContext(ctx)
	if templateTO.Err != nil {
		return nil, fmt.Errorf("get module template: %w", templateTO.Err)
	}
	return templateTO.ModuleTemplate, nil
}

func ModuleTemplateExists(ctx context.Context,
	clnt client.Client,
	module v1beta2.Module,
	defaultChannel string,
) error {
	moduleTemplate, err := GetModuleTemplate(ctx, clnt, module, defaultChannel)
	return CRExists(moduleTemplate, err)
}

func AllModuleTemplatesExists(ctx context.Context, clnt client.Client, kyma *v1beta2.Kyma) error {
	for _, module := range kyma.Spec.Modules {
		if err := ModuleTemplateExists(ctx, clnt, module, kyma.Spec.Channel); err != nil {
			return err
		}
	}

	return nil
}

func UpdateModuleTemplateSpec(ctx context.Context,
	clnt client.Client,
	module v1beta2.Module,
	newValue,
	kymaChannel string,
) error {
	moduleTemplate, err := GetModuleTemplate(ctx, clnt, module, kymaChannel)
	if err != nil {
		return err
	}
	if moduleTemplate.Spec.Data == nil {
		moduleTemplate.Spec.Data = &unstructured.Unstructured{}
	}
	moduleTemplate.Spec.Data.Object["spec"] = map[string]any{"initKey": newValue}
	return clnt.Update(ctx, moduleTemplate)
}
