package v1beta2_test

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func Test_ValidateVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		newVersion string
		oldVersion string
		wantErr    bool
	}{
		{
			name:       "valid version update due to version increment",
			newVersion: "v1.0.1",
			oldVersion: "v1.0.0",
			wantErr:    false,
		},
		{
			name:       "valid version update due to same version with different Prerelease",
			newVersion: "v1.0.0-RC2",
			oldVersion: "v1.0.0-RC1",
			wantErr:    false,
		},
		{
			name:       "valid version update due to same version with different Prerelease",
			newVersion: "v1.0.0-RC2",
			oldVersion: "v1.0.0-RC1",
			wantErr:    false,
		},
		{
			name:       "invalid version update due to version decrease",
			newVersion: "v1.0.0",
			oldVersion: "v1.0.1",
			wantErr:    true,
		},
		{
			name:       "invalid version update due to version decrease with Prerelease",
			newVersion: "v1.0.0-RC1",
			oldVersion: "v1.0.1-RC1",
			wantErr:    true,
		},
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			newVersion, _ := semver.NewVersion(testCase.newVersion)
			oldVersion, _ := semver.NewVersion(testCase.oldVersion)
			err := v1beta2.ValidateVersionUpgrade(newVersion, oldVersion, "test_template")
			if (err != nil) != testCase.wantErr {
				t.Errorf("ValidateVersionUpgrade() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}
