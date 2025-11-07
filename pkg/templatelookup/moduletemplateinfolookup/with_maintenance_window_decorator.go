package moduletemplateinfolookup

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
)

var (
	ErrWaitingForNextMaintenanceWindow = errors.New(
		"waiting for next maintenance window to update module version",
	)
	ErrFailedToDetermineIfMaintenanceWindowIsActive = errors.New("failed to determine if maintenance window is active")
)

type MaintenanceWindow interface {
	IsRequired(moduleTemplate *v1beta2.ModuleTemplate, kyma *v1beta2.Kyma) bool
	IsActive(kyma *v1beta2.Kyma) (bool, error)
}

type ModuleLookup interface {
	Lookup(ctx context.Context,
		moduleInfo *templatelookup.ModuleInfo,
		kyma *v1beta2.Kyma,
		moduleReleaseMeta *v1beta2.ModuleReleaseMeta,
	) templatelookup.ModuleTemplateInfo
}

type WithMaintenanceWindowDecorator struct {
	maintenanceWindow MaintenanceWindow
	lookup            ModuleLookup
}

func NewWithMaintenanceWindowDecorator(maintenanceWindow MaintenanceWindow,
	lookup ModuleLookup,
) WithMaintenanceWindowDecorator {
	return WithMaintenanceWindowDecorator{
		maintenanceWindow: maintenanceWindow,
		lookup:            lookup,
	}
}

func (p WithMaintenanceWindowDecorator) Lookup(ctx context.Context,
	moduleInfo *templatelookup.ModuleInfo,
	kyma *v1beta2.Kyma,
	moduleReleaseMeta *v1beta2.ModuleReleaseMeta,
) templatelookup.ModuleTemplateInfo {
	moduleTemplateInfo := p.lookup.Lookup(ctx,
		moduleInfo,
		kyma,
		moduleReleaseMeta)

	// lookup returns an error case => return immediately
	if moduleTemplateInfo.ModuleTemplate == nil || moduleTemplateInfo.Err != nil {
		return moduleTemplateInfo
	}

	if !p.maintenanceWindow.IsRequired(moduleTemplateInfo.ModuleTemplate, kyma) {
		return moduleTemplateInfo
	}

	active, err := p.maintenanceWindow.IsActive(kyma)
	if err != nil {
		moduleTemplateInfo.Err = fmt.Errorf("%w: %w", ErrFailedToDetermineIfMaintenanceWindowIsActive, err)
		moduleTemplateInfo.ModuleTemplate = nil
		return moduleTemplateInfo
	}

	if !active {
		moduleTemplateInfo.Err = ErrWaitingForNextMaintenanceWindow
		moduleTemplateInfo.ModuleTemplate = nil
		return moduleTemplateInfo
	}
	return moduleTemplateInfo
}
