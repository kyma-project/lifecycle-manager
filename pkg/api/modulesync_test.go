package api_test

import (
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

//nolint:funlen
func TestSyncEnabled(t *testing.T) {
	t.Parallel()

	t.Run("sync enabled by default for nil labels map", func(t *testing.T) {
		t.Parallel()
		module := v1beta2.ModuleTemplate{}
		module.Labels = nil
		actual := module.SyncEnabled(false, false)
		if !actual {
			t.Error("Incorrect SyncEnabled value")
		}
	})

	tests := []struct {
		name               string
		syncLabelValue     string
		betaLabelValue     string
		internalLabelValue string
		betaEnabled        bool
		internalEnabled    bool
		expected           bool
	}{
		{
			expected:           true,
			name:               "sync is enabled for missing or empty labels",
			syncLabelValue:     "",
			betaLabelValue:     "",
			internalLabelValue: "",
			betaEnabled:        false,
			internalEnabled:    false,
		},
		{
			expected:           true,
			name:               "sync is enabled for explicit label value",
			syncLabelValue:     "true",
			betaLabelValue:     "",
			internalLabelValue: "",
			betaEnabled:        false,
			internalEnabled:    false,
		},
		{
			expected:           false,
			name:               "sync is disabled for explicit label value",
			syncLabelValue:     "false",
			betaLabelValue:     "",
			internalLabelValue: "",
			betaEnabled:        false,
			internalEnabled:    false,
		},
		{
			expected:           false,
			name:               "beta sync is disabled by default",
			syncLabelValue:     "",
			betaLabelValue:     "true",
			internalLabelValue: "",
			betaEnabled:        false,
			internalEnabled:    false,
		},
		{
			expected:           true,
			name:               "beta sync is enabled if explicitly enabled",
			syncLabelValue:     "",
			betaLabelValue:     "true",
			internalLabelValue: "",
			betaEnabled:        true,
			internalEnabled:    false,
		},
		{
			expected:           false,
			name:               "internal sync is disabled by default",
			syncLabelValue:     "",
			betaLabelValue:     "",
			internalLabelValue: "true",
			betaEnabled:        false,
			internalEnabled:    false,
		},
		{
			expected:           true,
			name:               "internal sync is enabled if explicitly enabled",
			syncLabelValue:     "",
			betaLabelValue:     "",
			internalLabelValue: "true",
			betaEnabled:        false,
			internalEnabled:    true,
		},
		{
			expected:           false,
			name:               "beta+internal sync is disabled by default",
			syncLabelValue:     "",
			betaLabelValue:     "true",
			internalLabelValue: "true",
			betaEnabled:        false,
			internalEnabled:    false,
		},
		{
			expected:           false,
			name:               "beta+internal sync is disabled in only internal is enabled",
			syncLabelValue:     "",
			betaLabelValue:     "true",
			internalLabelValue: "true",
			betaEnabled:        false,
			internalEnabled:    true,
		},
		{
			expected:           true,
			name:               "beta+internal sync is enabled if both beta and internal are explicitly enabled",
			syncLabelValue:     "",
			betaLabelValue:     "true",
			internalLabelValue: "true",
			betaEnabled:        true,
			internalEnabled:    true,
		},
	}

	for _, testCase := range tests {
		tcase := testCase
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			module := v1beta2.ModuleTemplate{}
			module.Labels = map[string]string{}
			module.Labels[shared.SyncLabel] = tcase.syncLabelValue
			module.Labels[shared.BetaLabel] = tcase.betaLabelValue
			module.Labels[shared.InternalLabel] = tcase.internalLabelValue

			actual := module.SyncEnabled(tcase.betaEnabled, tcase.internalEnabled)
			if actual != tcase.expected {
				t.Error("Incorrect SyncEnabled value")
			}
		})
	}
}
