package maintenancewindows_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/maintenancewindows"
	"github.com/kyma-project/lifecycle-manager/maintenancewindows/resolver"
)

func TestMaintenancePolicyFileExists_FileNotExists(t *testing.T) {
	got := maintenancewindows.MaintenancePolicyFileExists("testdata/file.json")

	require.False(t, got)
}

func TestMaintenancePolicyFileExists_FileExists(t *testing.T) {
	got := maintenancewindows.MaintenancePolicyFileExists("testdata/policy.json")

	require.True(t, got)
}

func TestInitializeMaintenanceWindowsPolicy_FileNotExist_NoError(t *testing.T) {
	got, err := maintenancewindows.InitializeMaintenanceWindowsPolicy(logr.Logger{}, "testdata", "policy-1")

	require.Nil(t, got)
	require.NoError(t, err)
}

func TestInitializeMaintenanceWindowsPolicy_DirectoryNotExist_NoError(t *testing.T) {
	got, err := maintenancewindows.InitializeMaintenanceWindowsPolicy(logr.Logger{}, "files", "policy")

	require.Nil(t, got)
	require.NoError(t, err)
}

func TestInitializeMaintenanceWindowsPolicy_InvalidPolicy(t *testing.T) {
	got, err := maintenancewindows.InitializeMaintenanceWindowsPolicy(logr.Logger{}, "testdata", "invalid-policy")

	require.Nil(t, got)
	require.ErrorContains(t, err, "failed to get maintenance window policy")
}

func TestInitializeMaintenanceWindowsPolicy_WhenFileExists_CorrectPolicyIsRead(t *testing.T) {
	got, err := maintenancewindows.InitializeMaintenanceWindowsPolicy(logr.Logger{}, "testdata", "policy")

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
	require.Equal(t, expectedPolicy, got)
}

func parseTime(value string) (time.Time, error) {
	t, err := time.Parse("15:04:05Z07:00", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse time: %w", err)
	}

	return t, nil
}
