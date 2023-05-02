package v1beta1_test

import (
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
)

func TestSyncEnabled(t *testing.T) {
	t.Parallel()

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
			name:               "default case",
			syncLabelValue:     "",
			betaLabelValue:     "",
			internalLabelValue: "",
			betaEnabled:        false,
			internalEnabled:    false,
		},
		{
			expected:           true,
			name:               "default with explicit label value",
			syncLabelValue:     "true",
			betaLabelValue:     "",
			internalLabelValue: "",
			betaEnabled:        false,
			internalEnabled:    false,
		},
		{
			expected:           false,
			name:               "disable module",
			syncLabelValue:     "false",
			betaLabelValue:     "",
			internalLabelValue: "",
			betaEnabled:        false,
			internalEnabled:    false,
		},
		{
			expected:           false,
			name:               "beta module must be explicitly enabled",
			syncLabelValue:     "",
			betaLabelValue:     "true",
			internalLabelValue: "",
			betaEnabled:        false,
			internalEnabled:    false,
		},
		{
			expected:           true,
			name:               "beta module is explicitly enabled",
			syncLabelValue:     "",
			betaLabelValue:     "true",
			internalLabelValue: "",
			betaEnabled:        true,
			internalEnabled:    false,
		},
		{
			expected:           false,
			name:               "internal module must be explicitly enabled",
			syncLabelValue:     "",
			betaLabelValue:     "",
			internalLabelValue: "true",
			betaEnabled:        false,
			internalEnabled:    false,
		},
		{
			expected:           true,
			name:               "internal module is explicitly enabled",
			syncLabelValue:     "",
			betaLabelValue:     "",
			internalLabelValue: "true",
			betaEnabled:        false,
			internalEnabled:    true,
		},
		{
			expected:           false,
			name:               "beta+internal module must be explicitly enabled (1)",
			syncLabelValue:     "",
			betaLabelValue:     "true",
			internalLabelValue: "true",
			betaEnabled:        false,
			internalEnabled:    false,
		},
		{
			expected:           false,
			name:               "beta+internal module must be explicitly enabled (2)",
			syncLabelValue:     "",
			betaLabelValue:     "true",
			internalLabelValue: "true",
			betaEnabled:        false,
			internalEnabled:    true,
		},
		{
			expected:           true,
			name:               "sync may be enabled for beta+internal module",
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

			module := v1beta1.ModuleTemplate{}
			module.Labels = map[string]string{}
			module.Labels[v1beta1.SyncLabel] = tcase.syncLabelValue
			module.Labels[v1beta1.BetaLabel] = tcase.betaLabelValue
			module.Labels[v1beta1.InternalLabel] = tcase.internalLabelValue

			actual := module.SyncEnabled(tcase.betaEnabled, tcase.internalEnabled)
			if actual != tcase.expected {
				t.Error("Incorrect SyncEnabled value")
			}
		})
	}
}
