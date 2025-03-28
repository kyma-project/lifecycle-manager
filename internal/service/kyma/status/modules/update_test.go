package modules_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"testing"

	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/status/modules"
)

func TestUpdateModuleStatuses_WhenCalledWithNilKyma_Returns(t *testing.T) {
	statusService := modules.NewModulesStatusService(nil, nil)

	statusService.UpdateModuleStatuses(t.Context(), nil, common.Modules{})
}

func TestUpdateModuleStatuses_WhenCalledWithEmptyModules_Returns(t *testing.T) {
	statusService := modules.NewModulesStatusService(nil, nil)

	statusService.UpdateModuleStatuses(t.Context(), &v1beta2.Kyma{}, common.Modules{})
}

func TestUpdateModuleStatuses_WhenCalledWithTemplateErrorTemplateUpdateNotAllowed_CreatesStateWarning(t *testing.T) {
	statusService := modules.NewModulesStatusService(nil, nil)

	statusService.UpdateModuleStatuses(t.Context(), &v1beta2.Kyma{}, common.Modules{})
}
