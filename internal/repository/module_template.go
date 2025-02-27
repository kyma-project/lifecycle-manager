package repository

import (
	"context"
	"fmt"

	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type ModuleTemplateRepository struct {
	client.Client
}

func NewModuleTemplateRepository(client client.Client) *ModuleTemplateRepository {
	return &ModuleTemplateRepository{Client: client}
}

func (m *ModuleTemplateRepository) Get(ctx context.Context,
	namespacedName client.ObjectKey,
) (*v1beta2.ModuleTemplate, error) {
	template := &v1beta2.ModuleTemplate{}
	if err := m.Client.Get(ctx, namespacedName, template); err != nil {
		return nil, fmt.Errorf("failed to get ModuleTemplate: %w", err)
	}
	return template, nil
}

func (m *ModuleTemplateRepository) Update(ctx context.Context,
	template *v1beta2.ModuleTemplate,
) error {
	if err := m.Client.Update(ctx, template); err != nil {
		return fmt.Errorf("failed to update ModuleTemplate: %w", err)
	}
	return nil
}

func (m *ModuleTemplateRepository) ListByLabel(ctx context.Context,
	labelSelector k8slabels.Selector,
) (*v1beta2.ModuleTemplateList, error) {
	moduleTemplateList := &v1beta2.ModuleTemplateList{}
	if err := m.Client.List(ctx, moduleTemplateList,
		&client.ListOptions{LabelSelector: labelSelector}); err != nil {
		return nil, fmt.Errorf("could not list ModuleTemplates: %w", err)
	}
	return moduleTemplateList, nil
}
