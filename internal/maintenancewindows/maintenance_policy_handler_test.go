package maintenancewindows_test

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyma-project/lifecycle-manager/internal/maintenancewindows"
	"github.com/stretchr/testify/require"
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
	got, err := maintenancewindows.InitializeMaintenanceWindowsPolicy(logr.Logger{})

	require.Nil(t, got)
	require.NoError(t, err)
}
