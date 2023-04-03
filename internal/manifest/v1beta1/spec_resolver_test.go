package v1beta1_test

import (
	"reflect"
	"testing"

	"github.com/kyma-project/lifecycle-manager/internal/manifest/v1beta1"
)

//nolint:funlen
func Test_ParseInstallConfigs(t *testing.T) {
	t.Parallel()
	type args struct {
		decodedConfig interface{}
	}
	var emptyConfigs []interface{}

	// capacity 4 to mimic json.Unmarshal logic
	validConfigs := make([]interface{}, 0, 4)
	validConfigs = append(validConfigs,
		map[string]interface{}{
			"name":      "test",
			"overrides": "test2",
		})

	tests := []struct {
		name    string
		args    args
		want    []interface{}
		wantErr bool
	}{
		{
			name: "Empty config file",
			args: args{
				decodedConfig: nil,
			},
			want:    emptyConfigs,
			wantErr: false,
		},
		{
			name: "Empty configs object",
			args: args{
				decodedConfig: map[string]interface{}{
					"configs": nil,
				},
			},
			want:    emptyConfigs,
			wantErr: false,
		},
		{
			name: "Non empty configs with no configs object",
			args: args{
				decodedConfig: map[string]interface{}{
					"test": map[string]string{
						"name": "test",
					},
				},
			},
			want:    emptyConfigs,
			wantErr: false,
		},
		{
			name: "Valid configs",
			args: args{
				decodedConfig: map[string]interface{}{
					"configs": []interface{}{
						map[string]string{
							"name":      "test",
							"overrides": "test2",
						},
					},
				},
			},
			want:    validConfigs,
			wantErr: false,
		},
		{
			name: "Invalid configs",
			args: args{
				decodedConfig: map[string]interface{}{
					"configs": map[string]string{
						"name":      "test",
						"overrides": "test2",
					},
				},
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
