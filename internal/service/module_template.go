package service

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/repository"
)

type ModuleTemplateService struct {
	moduleTemplateRepository *repository.ModuleTemplateRepository
}

func NewModuleTemplateService(client client.Client) *ModuleTemplateService {
	return &ModuleTemplateService{
		moduleTemplateRepository: repository.NewModuleTemplateRepository(client),
	}
}

func (s *ModuleTemplateService) GetModuleTemplate(ctx context.Context,
	namespacedName client.ObjectKey) (*v1beta2.ModuleTemplate,
	error,
) {
	return s.moduleTemplateRepository.Get(ctx, namespacedName)
}

func (s *ModuleTemplateService) UpdateFinalizer(ctx context.Context,
	template *v1beta2.ModuleTemplate, finalizer string,
) (bool, error) {
	updateRequired := controllerutil.AddFinalizer(template, finalizer)
	if updateRequired {
		if err := s.moduleTemplateRepository.Update(ctx, template); err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}

func (s *ModuleTemplateService) RemoveFinalizer(ctx context.Context,
	template *v1beta2.ModuleTemplate, finalizer string,
) (bool, error) {
	updateRequired := controllerutil.RemoveFinalizer(template, finalizer)
	if updateRequired {
		if err := s.moduleTemplateRepository.Update(ctx, template); err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}
