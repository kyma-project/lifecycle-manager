package moduletemplateinfolookup

import (
	"context"
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
)

var (
	ErrWaitingForNextMaintenanceWindow              = errors.New("waiting for next maintenance window to update module version")
	ErrFailedToDetermineIfMaintenanceWindowIsActive = errors.New("failed to determine if maintenance window is active")
)

type MaintenanceWindow interface {
	IsRequired(moduleTemplate *v1beta2.ModuleTemplate, kyma *v1beta2.Kyma) bool
	IsActive(kyma *v1beta2.Kyma) (bool, error)
}

// ModuleTemplateInfoLookup looks up the module template via the module release meta.
type ModuleTemplateInfoLookup struct {
	client            client.Reader
	maintenanceWindow MaintenanceWindow
}

func New(client client.Reader) ModuleTemplateInfoLookup {
	return ModuleTemplateInfoLookup{client: client}
}

func NewWithMaintenanceWindow(client client.Reader, maintenanceWindow MaintenanceWindow) ModuleTemplateInfoLookup {
	return ModuleTemplateInfoLookup{
		client:            client,
		maintenanceWindow: maintenanceWindow,
	}
}

func (s ModuleTemplateInfoLookup) Lookup(ctx context.Context,
	moduleInfo *templatelookup.ModuleInfo,
	kyma *v1beta2.Kyma,
	moduleReleaseMeta *v1beta2.ModuleReleaseMeta,
) templatelookup.ModuleTemplateInfo {
	moduleTemplateInfo := templatelookup.ModuleTemplateInfo{}
	moduleTemplateInfo.DesiredChannel = getDesiredChannel(moduleInfo.Channel, kyma.Spec.Channel)

	var desiredModuleVersion string
	var err error
	if moduleReleaseMeta.Spec.Mandatory != nil {
		desiredModuleVersion, err = templatelookup.GetMandatoryVersionForModule(moduleReleaseMeta)
	} else {
		desiredModuleVersion, err = templatelookup.GetChannelVersionForModule(moduleReleaseMeta,
			moduleTemplateInfo.DesiredChannel)
	}
	if err != nil {
		moduleTemplateInfo.Err = err
		return moduleTemplateInfo
	}

	template, err := getTemplateByVersion(ctx,
		s.client,
		moduleInfo.Name,
		desiredModuleVersion,
		kyma.Namespace)
	if err != nil {
		moduleTemplateInfo.Err = err
		return moduleTemplateInfo
	}

	moduleTemplateInfo.ModuleTemplate = template

	// Apply maintenance window logic if configured
	if s.maintenanceWindow != nil {
		return s.applyMaintenanceWindowLogic(moduleTemplateInfo, kyma)
	}

	return moduleTemplateInfo
}

func (s ModuleTemplateInfoLookup) applyMaintenanceWindowLogic(
	moduleTemplateInfo templatelookup.ModuleTemplateInfo,
	kyma *v1beta2.Kyma,
) templatelookup.ModuleTemplateInfo {
	// If template lookup failed, return immediately
	if moduleTemplateInfo.ModuleTemplate == nil || moduleTemplateInfo.Err != nil {
		return moduleTemplateInfo
	}

	if !s.maintenanceWindow.IsRequired(moduleTemplateInfo.ModuleTemplate, kyma) {
		return moduleTemplateInfo
	}

	active, err := s.maintenanceWindow.IsActive(kyma)
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
