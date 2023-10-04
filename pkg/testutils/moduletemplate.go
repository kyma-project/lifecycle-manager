package testutils

import (
	"context"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
