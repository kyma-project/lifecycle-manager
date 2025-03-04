package skrresources //nolint:testpackage // testing package internals

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetAllowedManagers(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     []string
	}{
		{
			name:     "default managers",
			envValue: "",
			want:     []string{"declarative.kyma-project.io/applier", "lifecycle-manager", "k3s"},
		},
		{
			name:     "single manager in env",
			envValue: "manager1",
			want:     []string{"manager1"},
		},
		{
			name:     "multiple managers in env",
			envValue: "manager1;manager2;some-manager:3",
			want:     []string{"manager1", "manager2", "some-manager:3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv(knownManagersEnvVar, tt.envValue)
			}
			assert.Equal(t, tt.want, getAllowedManagers())
		})
	}
}

func TestGetCacheTTL(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     int
	}{
		{
			name:     "default TTL",
			envValue: "",
			want:     300,
		},
		{
			name:     "custom TTL",
			envValue: "123",
			want:     123,
		},
		{
			name:     "invalid value is ignored, default TTL is returned",
			envValue: "abc",
			want:     300,
		},
		{
			name:     "zero is invalid, default TTL is returned",
			envValue: "0",
			want:     300,
		},
		{
			name:     "Negative value is ignored, default TTL is returned",
			envValue: "-123",
			want:     300,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv(frequencyLimiterTTLEnvVar, tt.envValue)
			}
			assert.Equal(t, tt.want, getFrequencyLimiterTTL())
		})
	}
}
