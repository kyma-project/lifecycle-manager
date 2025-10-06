package modules_test

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/status/modules"
	modulecommon "github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

const (
	InvalidModulePrefix = "invalid_"
	ModuleShouldKeep    = "ModuleShouldKeep"
	ModuleToBeRemoved   = "ModuleToBeRemoved"
)

func TestMetricsOnDeleteNoLongerExistingModuleStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                         string
		ModuleInStatus               string
		getModule                    modules.GetModuleFunc
		expectModuleMetricsGetCalled bool
	}{
		{
			"When status.modules contains Manifest not found in cluster, expect RemoveModuleStateMetrics get called",
			ModuleToBeRemoved,
			moduleDeletedSuccessfullyMock,
			true,
		},
		{
			"When status.modules contains Manifest still exits in cluster, expect RemoveModuleStateMetrics not called",
			ModuleToBeRemoved,
			moduleStillExistsInClusterMock,
			false,
		},
		{
			"When status.modules contains not valid Manifest, expect RemoveModuleStateMetrics get called",
			InvalidModulePrefix + ModuleToBeRemoved,
			moduleStillExistsInClusterMock,
			true,
		},
		{
			"When status.modules contains module in spec.module, expect RemoveModuleStateMetrics not called",
			ModuleShouldKeep,
			moduleStillExistsInClusterMock,
			false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			kyma := testutils.NewTestKyma("test-kyma")
			configureModuleInKyma(kyma, []string{ModuleShouldKeep}, []string{testCase.ModuleInStatus})
			kymaMetrics := &KymaMockMetrics{}
			modules.DeleteNoLongerExistingModuleStatus(t.Context(), kyma, testCase.getModule,
				kymaMetrics.RemoveModuleStateMetrics)
			if testCase.expectModuleMetricsGetCalled {
				assert.Equal(t, 1, kymaMetrics.callCount)
			} else {
				assert.Equal(t, 0, kymaMetrics.callCount)
			}
		})
	}
}

func TestDeleteNoLongerExistingModuleStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                        string
		ModulesInKymaSpec           []string
		ModulesInKymaStatus         []string
		ExpectedModulesInKymaStatus []string
		getModule                   modules.GetModuleFunc
	}{
		{
			"When status.modules contains valid modules not in spec.module, expect removed and spec.module keep",
			[]string{ModuleShouldKeep},
			[]string{ModuleShouldKeep, ModuleToBeRemoved},
			[]string{ModuleShouldKeep},
			moduleDeletedSuccessfullyMock,
		},
		{
			"When status.modules contains invalid modules not in spec.module, expect removed and spec.module keep",
			[]string{ModuleShouldKeep},
			[]string{ModuleShouldKeep, InvalidModulePrefix + ModuleToBeRemoved},
			[]string{ModuleShouldKeep},
			moduleDeletedSuccessfullyMock,
		},
		{
			"When status.modules contains invalid modules in spec.module, expect keep",
			[]string{InvalidModulePrefix + ModuleShouldKeep},
			[]string{InvalidModulePrefix + ModuleShouldKeep, ModuleToBeRemoved},
			[]string{InvalidModulePrefix + ModuleShouldKeep},
			moduleDeletedSuccessfullyMock,
		},
		{
			"When status.modules contains valid modules not in spec.module, " +
				"expect keep if module still in cluster",
			[]string{ModuleShouldKeep},
			[]string{ModuleShouldKeep, ModuleToBeRemoved},
			[]string{ModuleShouldKeep, ModuleToBeRemoved},
			moduleStillExistsInClusterMock,
		},

		{
			"When status.modules contains invalid modules not in spec.module, expect removed and spec.module keep",
			[]string{ModuleShouldKeep},
			[]string{ModuleShouldKeep, InvalidModulePrefix + ModuleToBeRemoved},
			[]string{ModuleShouldKeep},
			moduleStillExistsInClusterMock,
		},
		{
			"When status.modules contains invalid modules in spec.module, expect keep",
			[]string{InvalidModulePrefix + ModuleShouldKeep},
			[]string{InvalidModulePrefix + ModuleShouldKeep, ModuleToBeRemoved},
			[]string{InvalidModulePrefix + ModuleShouldKeep, ModuleToBeRemoved},
			moduleStillExistsInClusterMock,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			kymaMetrics := &KymaMockMetrics{}
			kyma := testutils.NewTestKyma("test-kyma")
			configureModuleInKyma(kyma, testCase.ModulesInKymaSpec, testCase.ModulesInKymaStatus)
			modules.DeleteNoLongerExistingModuleStatus(t.Context(), kyma, testCase.getModule,
				kymaMetrics.RemoveModuleStateMetrics)
			var modulesInFinalModuleStatus []string
			for _, moduleStatus := range kyma.Status.Modules {
				modulesInFinalModuleStatus = append(modulesInFinalModuleStatus, moduleStatus.Name)
			}
			sort.Strings(testCase.ExpectedModulesInKymaStatus)
			sort.Strings(modulesInFinalModuleStatus)
			assert.Equal(t, testCase.ExpectedModulesInKymaStatus, modulesInFinalModuleStatus)
		})
	}
}

func configureModuleInKyma(
	kyma *v1beta2.Kyma,
	modulesInKymaSpec, modulesInKymaStatus []string,
) {
	for _, moduleName := range modulesInKymaSpec {
		module := v1beta2.Module{
			Name:    moduleName,
			Managed: true,
		}
		kyma.Spec.Modules = append(kyma.Spec.Modules, module)
	}
	for _, moduleName := range modulesInKymaStatus {
		manifest := &v1beta2.TrackingObject{}
		if strings.Contains(moduleName, InvalidModulePrefix) {
			manifest = nil
		}
		module := v1beta2.ModuleStatus{
			Name:     moduleName,
			Manifest: manifest,
		}
		kyma.Status.Modules = append(kyma.Status.Modules, module)
	}
}

func TestUpdateModuleStatuses_WhenCalledWithNilKyma_Returns(t *testing.T) {
	statusHandler := modules.NewStatusHandler(nil, nil, nil)

	_ = statusHandler.UpdateModuleStatuses(t.Context(), nil, modulecommon.Modules{})
}

func TestUpdateModuleStatuses_WhenCalledWithEmptyModules_Returns(t *testing.T) {
	statusHandler := modules.NewStatusHandler(nil, nil, nil)

	_ = statusHandler.UpdateModuleStatuses(t.Context(), &v1beta2.Kyma{}, modulecommon.Modules{})
}

func TestUpdateModuleStatuses_WhenCalledWithTemplateErrorTemplateUpdateNotAllowed_CreatesStateWarning(t *testing.T) {
	statusHandler := modules.NewStatusHandler(nil, nil, nil)

	_ = statusHandler.UpdateModuleStatuses(t.Context(), &v1beta2.Kyma{}, modulecommon.Modules{})
}

func TestUpdateModuleStatuses_WhenStatusGeneratorReturnsError_NotReturnsError_And_ModuleStatusExistsWithErrorState(
	t *testing.T,
) {
	const moduleStatusError = "status generator error"
	const moduleUnderTest = "test-module"
	statusGenerator := &mockStatusGenerator{
		generateModuleStatusFunc: func() (*v1beta2.ModuleStatus, error) {
			return nil, errors.New(moduleStatusError)
		},
	}
	statusHandler := modules.NewStatusHandler(statusGenerator, nil, nil)
	kyma := testutils.NewTestKyma("test-kyma")
	configureModuleInKyma(kyma, []string{moduleUnderTest}, nil)
	err := statusHandler.UpdateModuleStatuses(t.Context(), kyma, modulecommon.Modules{
		&modulecommon.Module{ModuleName: moduleUnderTest},
	})

	require.NoError(t, err)
	assert.Equal(t, shared.StateError, kyma.Status.Modules[0].State)
	assert.Equal(t, moduleStatusError, kyma.Status.Modules[0].Message)
	assert.Equal(t, moduleUnderTest, kyma.Status.Modules[0].Name)
}

func moduleDeletedSuccessfullyMock(_ context.Context, _ client.Object) error {
	return apierrors.NewNotFound(schema.GroupResource{}, "module-no-longer-exists")
}

func moduleStillExistsInClusterMock(_ context.Context, _ client.Object) error {
	return apierrors.NewAlreadyExists(schema.GroupResource{}, "module-still-exists")
}

type mockStatusGenerator struct {
	generateModuleStatusFunc func() (*v1beta2.ModuleStatus, error)
}

func (m *mockStatusGenerator) GenerateModuleStatus(_ *modulecommon.Module,
	_ *v1beta2.ModuleStatus,
) (*v1beta2.ModuleStatus, error) {
	return m.generateModuleStatusFunc()
}

type KymaMockMetrics struct {
	callCount int
}

func (m *KymaMockMetrics) RemoveModuleStateMetrics(kymaName, moduleName string) {
	m.callCount++
}
