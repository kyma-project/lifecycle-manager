package fromerror

import (
	"errors"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
)

var errFunctionCalledWitNilError = errors.New("can not generate a modulestatus without error")

func GenerateModuleStatusFromError(err error, moduleName, desiredChannel, fqdn string,
	status *v1beta2.ModuleStatus,
) (*v1beta2.ModuleStatus, error) {
	if err == nil {
		return nil, errFunctionCalledWitNilError
	}

	if status == nil {
		return newDefaultErrorStatus(moduleName, desiredChannel, fqdn, err), nil
	}

	if errorIsWaitingForMaintenanceWindow(err) {
		newModuleStatus := status.DeepCopy()
		newModuleStatus.Message = err.Error()
		newModuleStatus.Maintenance = true
		return newModuleStatus, nil
	}

	if errorIsMaintenanceWindowUnknown(err) {
		newModuleStatus := status.DeepCopy()
		newModuleStatus.Message = err.Error()
		newModuleStatus.State = shared.StateError
		return newModuleStatus, nil
	}

	if errorIsForbiddenTemplateUpdate(err) {
		newModuleStatus := status.DeepCopy()
		newModuleStatus.Message = err.Error()
		newModuleStatus.State = shared.StateWarning
		return newModuleStatus, nil
	}

	newStatus := newDefaultErrorStatus(moduleName, desiredChannel, fqdn, err)

	if errorIsTemplateNotFound(err) {
		newStatus.State = shared.StateError
	}

	return newStatus, nil
}

func errorIsWaitingForMaintenanceWindow(err error) bool {
	return errors.Is(err, moduletemplateinfolookup.ErrWaitingForNextMaintenanceWindow)
}

func errorIsMaintenanceWindowUnknown(err error) bool {
	return errors.Is(err, moduletemplateinfolookup.ErrFailedToDetermineIfMaintenanceWindowIsActive)
}

func errorIsForbiddenTemplateUpdate(err error) bool {
	return errors.Is(err, templatelookup.ErrTemplateUpdateNotAllowed)
}

func errorIsTemplateNotFound(err error) bool {
	return errors.Is(err, moduletemplateinfolookup.ErrNoTemplatesInListResult)
}

func newDefaultErrorStatus(moduleName, desiredChannel, fqdn string, err error) *v1beta2.ModuleStatus {
	return &v1beta2.ModuleStatus{
		Name:    moduleName,
		Channel: desiredChannel,
		FQDN:    fqdn,
		State:   shared.StateError,
		Message: err.Error(),
	}
}
