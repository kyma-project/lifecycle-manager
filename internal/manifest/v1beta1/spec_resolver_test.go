package v1beta1_test

import (
	"reflect"
	"testing"

	"github.com/kyma-project/lifecycle-manager/internal"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/v1beta1"
)

//nolint:funlen
func Test_ParseInstallConfigs(t *testing.T) {
	t.Parallel()
	type args struct {
		decodedConfig interface{}
	}
	var emptyConfigs []interface{}
	emptyFile, _ := internal.GetYamlFileContent("../../../pkg/test_samples/test-config-files/empty.yaml")
	nonEmptyConfig, _ := internal.GetYamlFileContent("../../../pkg/test_samples/test-config-files/non-empty-configs.yaml")
	emptyConfig, _ := internal.GetYamlFileContent("../../../pkg/test_samples/test-config-files/empty-configs.yaml")
	invalidConfig, _ := internal.GetYamlFileContent("../../../pkg/test_samples/test-config-files/invalid-configs.yaml")

	tests := []struct {
		name    string
		args    args
		want    []interface{}
		wantErr bool
	}{
		{
			name: "Empty config file",
			args: args{
				decodedConfig: emptyFile,
			},
			want:    emptyConfigs,
			wantErr: false,
		},
		{
			name: "Empty configs object",
			args: args{
				decodedConfig: emptyConfig,
			},
			want:    emptyConfigs,
			wantErr: false,
		},
		{
			name: "Valid configs",
			args: args{
				decodedConfig: nonEmptyConfig,
			},
			want: []interface{}{
				map[string]interface{}{
					"name":      "test",
					"overrides": "test2",
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid configs",
			args: args{
				decodedConfig: invalidConfig,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, testCase := range tests {
		tcase := testCase
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()
			got, err := v1beta1.ParseInstallConfigs(tcase.args.decodedConfig)
			if (err != nil) != tcase.wantErr {
				t.Errorf("parseInstallConfigs() error = %v, wantErr %v", err, tcase.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tcase.want) {
				t.Errorf("parseInstallConfigs() got = %v, want %v", got, tcase.want)
			}
		})
	}
}
