package maintenancewindows_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/maintenancewindows"
	"github.com/kyma-project/lifecycle-manager/maintenancewindows/resolver"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestMaintenancePolicyFileExists_FileNotExists(t *testing.T) {
	got := maintenancewindows.MaintenancePolicyFileExists("testdata/file.json")

	require.False(t, got)
}

func TestMaintenancePolicyFileExists_FileExists(t *testing.T) {
	got := maintenancewindows.MaintenancePolicyFileExists("testdata/policy.json")

	require.True(t, got)
}

func TestInitializeMaintenanceWindowsPolicy_FileNotExist(t *testing.T) {
	got, err := maintenancewindows.InitializeMaintenanceWindow(logr.Logger{},
		"testdata",
		"policy-1",
		20*time.Minute)

	require.Nil(t, got.MaintenanceWindowPolicy)
	require.ErrorContains(t, err, maintenancewindows.ErrPolicyFileNotFound.Error())
}

func TestInitializeMaintenanceWindowsPolicy_DirectoryNotExist(t *testing.T) {
	got, err := maintenancewindows.InitializeMaintenanceWindow(logr.Logger{},
		"files",
		"policy",
		20*time.Minute)

	require.Nil(t, got.MaintenanceWindowPolicy)
	require.ErrorContains(t, err, maintenancewindows.ErrPolicyFileNotFound.Error())
}

func TestInitializeMaintenanceWindowsPolicy_InvalidPolicy(t *testing.T) {
	got, err := maintenancewindows.InitializeMaintenanceWindow(logr.Logger{},
		"testdata",
		"invalid-policy",
		20*time.Minute)

	require.Nil(t, got.MaintenanceWindowPolicy)
	require.ErrorContains(t, err, "failed to get maintenance window policy")
}

func TestInitializeMaintenanceWindowsPolicy_WhenFileExists_CorrectPolicyIsRead(t *testing.T) {
	got, err := maintenancewindows.InitializeMaintenanceWindow(logr.Logger{},
		"testdata",
		"policy",
		20*time.Minute)
	require.NoError(t, err)

	ruleOneBeginTime, err := parseTime("01:00:00+00:00")
	require.NoError(t, err)
	ruleOneEndTime, err := parseTime("01:00:00+00:00")
	require.NoError(t, err)

	ruleTwoBeginTime, err := parseTime("21:00:00+00:00")
	require.NoError(t, err)
	ruleTwoEndTime, err := parseTime("00:00:00+00:00")
	require.NoError(t, err)

	defaultBeginTime, err := parseTime("21:00:00+00:00")
	require.NoError(t, err)
	defaultEndTime, err := parseTime("23:00:00+00:00")
	require.NoError(t, err)

	expectedPolicy := &resolver.MaintenanceWindowPolicy{
		Rules: []resolver.MaintenancePolicyRule{
			{
				Match: resolver.MaintenancePolicyMatch{
					Plan: resolver.NewRegexp("trial|free"),
				},
				Windows: resolver.MaintenanceWindows{
					{
						Days:  []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
						Begin: resolver.WindowTime(ruleOneBeginTime),
						End:   resolver.WindowTime(ruleOneEndTime),
					},
				},
			},
			{
				Match: resolver.MaintenancePolicyMatch{
					Region: resolver.NewRegexp("europe|eu-|uksouth"),
				},
				Windows: resolver.MaintenanceWindows{
					{
						Days:  []string{"Sat"},
						Begin: resolver.WindowTime(ruleTwoBeginTime),
						End:   resolver.WindowTime(ruleTwoEndTime),
					},
				},
			},
		},
		Default: resolver.MaintenanceWindow{
			Days:  []string{"Sat"},
			Begin: resolver.WindowTime(defaultBeginTime),
			End:   resolver.WindowTime(defaultEndTime),
		},
	}

	require.NoError(t, err)
	require.Equal(t, expectedPolicy, got.MaintenanceWindowPolicy)
}

func parseTime(value string) (time.Time, error) {
	t, err := time.Parse("15:04:05Z07:00", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse time: %w", err)
	}

	return t, nil
}

var installedModuleStatus = v1beta2.ModuleStatus{
	Name:    "module-name",
	Version: "1.0.0",
}

func Test_IsRequired_Returns_False_WhenNotRequiringDowntime(t *testing.T) {
	maintenanceWindow := maintenancewindows.MaintenanceWindow{
		MaintenanceWindowPolicy: maintenanceWindowInactiveStub{},
	}

	kyma := builder.NewKymaBuilder().
		WithModuleStatus(installedModuleStatus).
		Build()
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithVersion("2.0.0").
		WithModuleName(installedModuleStatus.Name).
		WithRequiresDowntime(false).
		Build()

	result := maintenanceWindow.IsRequired(moduleTemplate, kyma)

	assert.False(t, result)
}

func Test_IsRequired_Returns_False_WhenSkippingMaintenanceWindows(t *testing.T) {
	maintenanceWindow := maintenancewindows.MaintenanceWindow{
		MaintenanceWindowPolicy: maintenanceWindowInactiveStub{},
	}

	kyma := builder.NewKymaBuilder().
		WithModuleStatus(installedModuleStatus).
		WithSkipMaintenanceWindows(true).
		Build()
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithVersion("2.0.0").
		WithModuleName(installedModuleStatus.Name).
		WithRequiresDowntime(true).
		Build()

	result := maintenanceWindow.IsRequired(moduleTemplate, kyma)

	assert.False(t, result)
}

func Test_IsRequired_Returns_False_WhenModuleIsNotInstalledYet(t *testing.T) {
	maintenanceWindow := maintenancewindows.MaintenanceWindow{
		MaintenanceWindowPolicy: maintenanceWindowInactiveStub{},
	}

	kyma := builder.NewKymaBuilder().
		WithSkipMaintenanceWindows(false).
		Build()
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithVersion("2.0.0").
		WithModuleName(installedModuleStatus.Name).
		WithRequiresDowntime(false).
		Build()

	result := maintenanceWindow.IsRequired(moduleTemplate, kyma)

	assert.False(t, result)
}

func Test_IsRequired_Returns_False_WhenSameVersionIsAlreadyInstalled(t *testing.T) {
	maintenanceWindow := maintenancewindows.MaintenanceWindow{
		MaintenanceWindowPolicy: maintenanceWindowInactiveStub{},
	}

	kyma := builder.NewKymaBuilder().
		WithModuleStatus(installedModuleStatus).
		WithSkipMaintenanceWindows(false).
		Build()
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithVersion("1.0.0").
		WithModuleName(installedModuleStatus.Name).
		WithRequiresDowntime(true).
		Build()

	result := maintenanceWindow.IsRequired(moduleTemplate, kyma)

	assert.False(t, result)
}

func Test_IsRequired_Returns_True_WhenMaintenanceWindowIsRequire(t *testing.T) {
	maintenanceWindow := maintenancewindows.MaintenanceWindow{
		MaintenanceWindowPolicy: maintenanceWindowInactiveStub{},
	}

	kyma := builder.NewKymaBuilder().
		WithModuleStatus(installedModuleStatus).
		WithSkipMaintenanceWindows(false).
		Build()
	moduleTemplate := builder.NewModuleTemplateBuilder().
		WithVersion("2.0.0").
		WithModuleName(installedModuleStatus.Name).
		WithRequiresDowntime(true).
		Build()

	result := maintenanceWindow.IsRequired(moduleTemplate, kyma)

	assert.True(t, result)
}

func Test_IsActive_Returns_Error_WhenResolvingMaintenanceWindowPolicyFails(t *testing.T) {
	maintenanceWindow := maintenancewindows.MaintenanceWindow{
		MaintenanceWindowPolicy: maintenanceWindowErrorStub{},
	}

	kyma := builder.NewKymaBuilder().Build()

	result, err := maintenanceWindow.IsActive(kyma)

	assert.False(t, result)
	require.Error(t, err)
}

func Test_IsActive_Returns_False_WhenOutsideMaintenanceWindow(t *testing.T) {
	maintenanceWindow := maintenancewindows.MaintenanceWindow{
		MaintenanceWindowPolicy: maintenanceWindowInactiveStub{},
	}

	kyma := builder.NewKymaBuilder().Build()

	result, err := maintenanceWindow.IsActive(kyma)

	assert.False(t, result)
	require.NoError(t, err)
}

func Test_IsActive_Returns_True_WhenInsideMaintenanceWindow(t *testing.T) {
	maintenanceWindow := maintenancewindows.MaintenanceWindow{
		MaintenanceWindowPolicy: maintenanceWindowActiveStub{},
	}

	kyma := builder.NewKymaBuilder().Build()

	result, err := maintenanceWindow.IsActive(kyma)

	assert.True(t, result)
	require.NoError(t, err)
}

func Test_IsActive_PassesRuntimeArgumentCorrectly(t *testing.T) {
	receivedRuntime := resolver.Runtime{}
	maintenanceWindowPolicyStub := maintenanceWindowRuntimeArgStub{
		receivedRuntime: &receivedRuntime,
	}
	maintenanceWindow := maintenancewindows.MaintenanceWindow{
		MaintenanceWindowPolicy: maintenanceWindowPolicyStub,
	}

	runtime := resolver.Runtime{
		GlobalAccountID: random.Name(),
		Region:          random.Name(),
		PlatformRegion:  random.Name(),
		Plan:            random.Name(),
	}
	kyma := builder.NewKymaBuilder().
		WithLabel(shared.GlobalAccountIDLabel, runtime.GlobalAccountID).
		WithLabel(shared.RegionLabel, runtime.Region).
		WithLabel(shared.PlatformRegionLabel, runtime.PlatformRegion).
		WithLabel(shared.PlanLabel, runtime.Plan).
		Build()

	result, err := maintenanceWindow.IsActive(kyma)

	assert.False(t, result)
	require.NoError(t, err)
	assert.Equal(t, runtime, receivedRuntime)
}

func Test_IsActive_Returns_False_And_Error_WhenNoPolicyConfigured(t *testing.T) {
	maintenanceWindow := maintenancewindows.MaintenanceWindow{
		MaintenanceWindowPolicy: nil,
	}

	kyma := builder.NewKymaBuilder().Build()

	result, err := maintenanceWindow.IsActive(kyma)

	assert.False(t, result)
	require.ErrorIs(t, err, maintenancewindows.ErrNoMaintenanceWindowPolicyConfigured)
}

// test stubs

type maintenanceWindowInactiveStub struct{}

func (s maintenanceWindowInactiveStub) Resolve(runtime *resolver.Runtime,
	opts ...interface{},
) (*resolver.ResolvedWindow, error) {
	return &resolver.ResolvedWindow{
		Begin: time.Now().Add(1 * time.Hour),
		End:   time.Now().Add(2 * time.Hour),
	}, nil
}

type maintenanceWindowActiveStub struct{}

func (s maintenanceWindowActiveStub) Resolve(runtime *resolver.Runtime, opts ...interface{}) (*resolver.ResolvedWindow,
	error,
) {
	return &resolver.ResolvedWindow{
		Begin: time.Now().Add(-1 * time.Hour),
		End:   time.Now().Add(1 * time.Hour),
	}, nil
}

type maintenanceWindowErrorStub struct{}

func (s maintenanceWindowErrorStub) Resolve(runtime *resolver.Runtime, opts ...interface{}) (*resolver.ResolvedWindow,
	error,
) {
	return &resolver.ResolvedWindow{}, errors.New("test error")
}

type maintenanceWindowRuntimeArgStub struct {
	receivedRuntime *resolver.Runtime
}

func (s maintenanceWindowRuntimeArgStub) Resolve(runtime *resolver.Runtime,
	opts ...interface{},
) (*resolver.ResolvedWindow, error) {
	*s.receivedRuntime = *runtime

	return &resolver.ResolvedWindow{}, nil
}
