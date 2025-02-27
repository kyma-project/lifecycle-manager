package repository

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type ModuleTemplateRepository struct {
	Client client.Client
}

func (r *ModuleTemplateRepository) GetModuleTemplate(ctx context.Context,
	namespacedName client.ObjectKey) (*v1beta2.ModuleTemplate, error) {
	template := &v1beta2.ModuleTemplate{}
	if err := r.Client.Get(ctx, namespacedName, template); err != nil {
		return nil, err
	}
	return template, nil
}
