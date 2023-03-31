package v1beta1

import (
	"reflect"
	"testing"
)

func Test_parseInstallConfigs(t *testing.T) {
	type args struct {
		decodedConfig interface{}
	}
	var emptyConfigs []interface{}

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
			want: []interface{}{
				map[string]string{
					"name":      "test",
					"overrides": "test2",
				},
			},
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseInstallConfigs(tt.args.decodedConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseInstallConfigs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseInstallConfigs() got = %v, want %v", got, tt.want)
			}
		})
	}
}
