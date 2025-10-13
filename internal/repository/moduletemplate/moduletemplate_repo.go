package moduletemplate

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type Repository struct {
	clnt      client.Client
	namespace string
}

func NewRepository(clnt client.Client, namespace string) *Repository {
	return &Repository{
		clnt:      clnt,
		namespace: namespace,
	}
}

func (r *Repository) ListAllForModule(ctx context.Context, moduleName string) ([]v1beta2.ModuleTemplate, error) {
	var moduleTemplateList v1beta2.ModuleTemplateList
	if err := r.clnt.List(ctx, &moduleTemplateList, client.InNamespace(r.namespace),
		client.MatchingLabels{shared.ModuleName: moduleName}); err != nil {
		return nil, fmt.Errorf("failed to list ModuleTemplates for module %s: %w", moduleName, err)
	}
	return moduleTemplateList.Items, nil
}

func (r *Repository) EnsureFinalizer(ctx context.Context, moduleTemplateName string, finalizer string) error {
	moduleTemplate, err := r.Get(ctx, moduleTemplateName)
	if err != nil {
		return err
	}
	if updated := controllerutil.AddFinalizer(moduleTemplate, finalizer); updated {
		if err := r.clnt.Update(ctx, moduleTemplate); err != nil {
			return fmt.Errorf("failed to add finalizer to ModuleTemplate %s: %w", moduleTemplateName, err)
		}
	}
	return nil
}

func (r *Repository) RemoveFinalizer(ctx context.Context, moduleTemplateName string, finalizer string) error {
	moduleTemplate, err := r.Get(ctx, moduleTemplateName)
	if err != nil {
		return err
	}
	if updated := controllerutil.RemoveFinalizer(moduleTemplate, finalizer); updated {
		if err := r.clnt.Update(ctx, moduleTemplate); err != nil {
			return fmt.Errorf("failed to remove finalizer from ModuleTemplate %s: %w", moduleTemplateName, err)
		}
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, moduleTemplateName string) (*v1beta2.ModuleTemplate, error) {
	moduleTemplate := &v1beta2.ModuleTemplate{}
	err := r.clnt.Get(ctx, client.ObjectKey{Name: moduleTemplateName, Namespace: r.namespace}, moduleTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to get ModuleTemplate %s in namespace %s: %w", moduleTemplateName, r.namespace,
			err)
	}
	return moduleTemplate, nil
}
