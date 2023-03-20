package v1beta1

import (
	"github.com/Masterminds/semver/v3"
	"testing"
)

func Test_validateVersion(t *testing.T) {
	type args struct {
		newVersion string
		oldVersion string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "valid version change due to version increment",
			args: args{
				oldVersion: "v1.0.0",
				newVersion: "v1.0.1",
			},
			wantErr: false,
		},
		{
			name: "valid version change due to same version with different Prerelease",
			args: args{
				oldVersion: "v1.0.0-RC1",
				newVersion: "v1.0.0-RC2",
			},
			wantErr: false,
		},
		{
			name: "valid version change due to same version with different Prerelease",
			args: args{
				newVersion: "v1.0.0-RC2",
				oldVersion: "v1.0.0-RC1",
			},
			wantErr: false,
		},
		{
			name: "invalid version change due to version decrease",
			args: args{
				newVersion: "v1.0.0",
				oldVersion: "v1.0.1",
			},
			wantErr: true,
		},
		{
			name: "invalid version change due to version decrease with Prerelease",
			args: args{
				newVersion: "v1.0.0-RC1",
				oldVersion: "v1.0.1-RC1",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			newVersion, err := semver.NewVersion(tt.args.newVersion)
			if (err != nil) != tt.wantErr {
				t.Errorf("semver.NewVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
			oldVersion, err := semver.NewVersion(tt.args.oldVersion)
			if (err != nil) != tt.wantErr {
				t.Errorf("semver.NewVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
			errs := validateVersion(newVersion, oldVersion, "test_template")
			if (errs != nil) != tt.wantErr {
				t.Errorf("validateVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
