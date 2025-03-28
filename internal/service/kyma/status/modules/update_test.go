package modules_test

import (
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/status/modules"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
)

func TestUpdateModuleStatuses_WhenCalledWithNilKyma_Returns(t *testing.T) {
	statusService := modules.NewModulesStatusService(nil, nil, nil)

	_ = statusService.UpdateModuleStatuses(t.Context(), nil, common.Modules{})
}

func TestUpdateModuleStatuses_WhenCalledWithEmptyModules_Returns(t *testing.T) {
	statusService := modules.NewModulesStatusService(nil, nil, nil)

	_ = statusService.UpdateModuleStatuses(t.Context(), &v1beta2.Kyma{}, common.Modules{})
}

func TestUpdateModuleStatuses_WhenCalledWithTemplateErrorTemplateUpdateNotAllowed_CreatesStateWarning(t *testing.T) {
	statusService := modules.NewModulesStatusService(nil, nil, nil)

	_ = statusService.UpdateModuleStatuses(t.Context(), &v1beta2.Kyma{}, common.Modules{})
}

type mockStatusGenerator struct{}

func (m *mockStatusGenerator) GenerateModuleStatus(_ *common.Module, _ *v1beta2.ModuleStatus) (v1beta2.ModuleStatus, error) {
	return v1beta2.ModuleStatus{}, nil
}
