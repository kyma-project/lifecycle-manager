package sync_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/module/sync"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

const (
	InvalidModulePrefix = "invalid_"
	ModuleShouldKeep    = "ModuleShouldKeep"
	ModuleToBeRemoved   = "ModuleToBeRemoved"
)

func moduleDeletedSuccessfullyMock(_ context.Context, _ client.Object) error {
	return apierrors.NewNotFound(schema.GroupResource{}, "module-no-longer-exists")
}

func moduleStillExistsInClusterMock(_ context.Context, _ client.Object) error {
	return apierrors.NewAlreadyExists(schema.GroupResource{}, "module-still-exists")
}

type ModuleMockMetrics struct {
	mock.Mock
}

func (m *ModuleMockMetrics) RemoveModuleStateMetrics(kymaName, moduleName string) {
	m.Called(kymaName, moduleName)
}

func TestMetricsOnDeleteNoLongerExistingModuleStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                         string
		ModuleInStatus               string
		getModule                    sync.GetModuleFunc
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
			mockMetrics := &ModuleMockMetrics{}
			const methodToBeCalled = "RemoveModuleStateMetrics"
			mockMetrics.On(methodToBeCalled, kyma.Name, testCase.ModuleInStatus).Return()
			sync.DeleteNoLongerExistingModuleStatus(context.TODO(), kyma, testCase.getModule, mockMetrics)
			if testCase.expectModuleMetricsGetCalled {
				mockMetrics.AssertCalled(t, methodToBeCalled, kyma.Name, testCase.ModuleInStatus)
			} else {
				mockMetrics.AssertNotCalled(t, methodToBeCalled, kyma.Name, testCase.ModuleInStatus)
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
		getModule                   sync.GetModuleFunc
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
			kyma := testutils.NewTestKyma("test-kyma")
			configureModuleInKyma(kyma, testCase.ModulesInKymaSpec, testCase.ModulesInKymaStatus)
			sync.DeleteNoLongerExistingModuleStatus(context.TODO(), kyma, testCase.getModule, nil)
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
			Name: moduleName,
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

func TestNeedToUpdate(t *testing.T) {
	type args struct {
		manifestInCluster  *v1beta2.Manifest
		newManifest        *v1beta2.Manifest
		moduleStatus       *v1beta2.ModuleStatus
		templateGeneration int64
	}
	const trackedModuleTemplateGeneration = 1
	const updatedModuleTemplateGeneration = 2
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"When manifest in cluster is nil, expect need to update",
			args{nil, &v1beta2.Manifest{}, &v1beta2.ModuleStatus{}, trackedModuleTemplateGeneration},
			true,
		},
		{
			"When new module version available, expect need to update",
			args{
				builder.NewManifestBuilder().WithName("test").WithVersion("0.1").WithChannel(
					v1beta2.DefaultChannel).Build(),
				builder.NewManifestBuilder().WithName("test").WithVersion("0.2").WithChannel(
					v1beta2.DefaultChannel).Build(),
				&v1beta2.ModuleStatus{
					Version: "0.1",
					Channel: "regular",
					Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: trackedModuleTemplateGeneration,
						},
					},
				},
				trackedModuleTemplateGeneration,
			},
			true,
		},
		{
			"When new module version available and module is mandatory module, expect need to update",
			args{
				builder.NewManifestBuilder().WithName("test").WithVersion("0.1").WithChannel(
					v1beta2.DefaultChannel).IsMandatoryModule().Build(),
				builder.NewManifestBuilder().WithName("test").WithVersion("0.2").WithChannel(
					v1beta2.DefaultChannel).IsMandatoryModule().Build(),
				nil,
				trackedModuleTemplateGeneration,
			},
			true,
		},
		{
			"When channel switch, expect need to update",
			args{
				builder.NewManifestBuilder().WithName("test").WithVersion("0.1").WithChannel(
					v1beta2.DefaultChannel).Build(),
				builder.NewManifestBuilder().WithName("test").WithVersion("0.1").WithChannel(
					"fast").Build(),
				&v1beta2.ModuleStatus{
					Version: "0.1", Channel: "regular", Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: trackedModuleTemplateGeneration,
						},
					},
				},
				trackedModuleTemplateGeneration,
			},
			true,
		},
		{
			"When no update required, expect no update",
			args{
				builder.NewManifestBuilder().WithName("test").WithVersion("0.1").WithChannel(
					v1beta2.DefaultChannel).Build(),
				builder.NewManifestBuilder().WithName("test").WithVersion("0.1").WithChannel(
					v1beta2.DefaultChannel).Build(),
				&v1beta2.ModuleStatus{
					State: "Ready", Version: "0.1", Channel: "regular", Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: trackedModuleTemplateGeneration,
						},
					},
				}, trackedModuleTemplateGeneration,
			},
			false,
		},
		{
			"When moduleTemplate Generation updated, expect update",
			args{
				builder.NewManifestBuilder().WithName("test").WithVersion("0.1").WithChannel(
					v1beta2.DefaultChannel).Build(),
				builder.NewManifestBuilder().WithName("test").WithVersion("0.1").WithChannel(
					v1beta2.DefaultChannel).Build(),
				&v1beta2.ModuleStatus{
					State: "Ready", Version: "0.1", Channel: "regular", Template: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Generation: trackedModuleTemplateGeneration,
						},
					},
				}, updatedModuleTemplateGeneration,
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, sync.NeedToUpdate(tt.args.manifestInCluster, tt.args.newManifest,
				tt.args.moduleStatus, tt.args.templateGeneration), "needToUpdate(%v, %v, %v)",
				tt.args.manifestInCluster, tt.args.newManifest,
				tt.args.moduleStatus)
		})
	}
}
