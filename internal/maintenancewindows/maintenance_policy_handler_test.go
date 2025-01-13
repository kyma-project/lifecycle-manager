package maintenancewindows_test

import (
	"testing"

	"github.com/kyma-project/lifecycle-manager/internal/maintenancewindows"
	"github.com/stretchr/testify/require"
)

func TestMaintenancePolicyFileExists_FileNotExists(t *testing.T) {
	got := maintenancewindows.MaintenancePolicyFileExists("testdata/file.json")

	require.False(t, got)
}

func TestMaintenancePolicyFileExists_FileExists(t *testing.T) {
	got := maintenancewindows.MaintenancePolicyFileExists("testdata/with-default.json")

	require.True(t, got)
}
