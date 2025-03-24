package modules_test

import (
	"context"
	"errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/status/modules"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
)

func TestStatusService_UpdateStatusModule(t *testing.T) {
	getModuleFunc := func(ctx context.Context, module client.Object) error {
		return nil
	}
	removeMetricsFunc := func(kymaName, moduleName string) {}

	statusService := modules.NewModulesStatusService(getModuleFunc, removeMetricsFunc)

	t.Run("update module status from existing modules", func(t *testing.T) {
		kyma := &v1beta2.Kyma{}
		modules := common.Modules{
			{
				ModuleName: "test-module",
				Template:   &templatelookup.ModuleTemplateInfo{},
				Manifest:   &v1beta2.Manifest{},
			},
		}

		statusService.UpdateStatusModule(t.Context(), kyma, modules)

		assert.Len(t, kyma.Status.Modules, 1)
		assert.Equal(t, "test-module", kyma.Status.Modules[0].Name)
	})

	t.Run("delete no longer existing module status", func(t *testing.T) {
		kyma := &v1beta2.Kyma{
			Status: v1beta2.KymaStatus{
				Modules: []v1beta2.ModuleStatus{
					{
						Name: "test-module",
						Manifest: &v1beta2.TrackingObject{
							PartialMeta: v1beta2.PartialMeta{
								Name:      "test-manifest",
								Namespace: "default",
							},
						},
					},
				},
			},
		}

		getModuleFunc = func(ctx context.Context, module client.Object) error {
			return apierrors.NewNotFound(schema.GroupResource{}, "module-no-longer-exists")
		}
		statusService = modules.NewModulesStatusService(getModuleFunc, removeMetricsFunc)

		statusService.UpdateStatusModule(context.Background(), kyma, nil)

		assert.Len(t, kyma.Status.Modules, 0)
	})

	t.Run("generate module status from error", func(t *testing.T) {
		kyma := &v1beta2.Kyma{}
		modules := common.Modules{
			{
				ModuleName: "test-module",
				Template:   &templatelookup.ModuleTemplateInfo{Err: errors.New("test error")},
				Manifest:   &v1beta2.Manifest{},
			},
		}

		statusService.UpdateStatusModule(context.Background(), kyma, modules)

		assert.Len(t, kyma.Status.Modules, 1)
		assert.Equal(t, "test-module", kyma.Status.Modules[0].Name)
		assert.Equal(t, shared.StateError, kyma.Status.Modules[0].State)
		assert.Equal(t, "test error", kyma.Status.Modules[0].Message)
	})

	t.Run("handle different template errors", func(t *testing.T) {
		tests := []struct {
			name          string
			templateError error
			expectedState shared.State
		}{
			{
				name:          "template update not allowed",
				templateError: templatelookup.ErrTemplateUpdateNotAllowed,
				expectedState: shared.StateWarning,
			},
			{
				name:          "no templates in list result",
				templateError: moduletemplateinfolookup.ErrNoTemplatesInListResult,
				expectedState: shared.StateWarning,
			},
			{
				name:          "waiting for next maintenance window",
				templateError: moduletemplateinfolookup.ErrWaitingForNextMaintenanceWindow,
				expectedState: shared.StateWarning,
			},
			{
				name:          "failed to determine if maintenance window is active",
				templateError: moduletemplateinfolookup.ErrFailedToDetermineIfMaintenanceWindowIsActive,
				expectedState: shared.StateError,
			},
			{
				name:          "default error case",
				templateError: errors.New("default error"),
				expectedState: shared.StateError,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				kyma := &v1beta2.Kyma{}
				modules := common.Modules{
					{
						ModuleName: "test-module",
						Template:   &templatelookup.ModuleTemplateInfo{Err: tt.templateError},
						Manifest:   &v1beta2.Manifest{},
					},
				}

				statusService.UpdateStatusModule(context.Background(), kyma, modules)

				assert.Len(t, kyma.Status.Modules, 1)
				assert.Equal(t, "test-module", kyma.Status.Modules[0].Name)
				assert.Equal(t, tt.expectedState, kyma.Status.Modules[0].State)
				assert.Equal(t, tt.templateError.Error(), kyma.Status.Modules[0].Message)
			})
		}
	})
}
