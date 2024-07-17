package v1beta2

import (
	"reflect"
	"testing"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_GetAvailableModules_When_ModuleInSpecOnly(t *testing.T) {
	tests := []struct {
		name     string
		KymaSpec KymaSpec
		want     []AvailableModule
	}{
		{
			name: "When Channel and Version are both set, then the module is invalid",
			KymaSpec: KymaSpec{
				Modules: []Module{
					{Name: "Module1", Channel: "regular", Version: "v1.0"},
				},
			},
			want: []AvailableModule{
				{Module: Module{Name: "Module1", Channel: "regular", Version: "v1.0"}, Enabled: true, Valid: false},
			},
		},
		{
			name: "When Channel is set, then the module is valid",
			KymaSpec: KymaSpec{
				Modules: []Module{
					{Name: "Module1", Channel: "regular"},
				},
			},
			want: []AvailableModule{
				{Module: Module{Name: "Module1", Channel: "regular"}, Enabled: true, Valid: true},
			},
		},
		{
			name: "When Version is set, then the module is valid",
			KymaSpec: KymaSpec{
				Modules: []Module{
					{Name: "Module1", Version: "v1.0"},
				},
			},
			want: []AvailableModule{
				{Module: Module{Name: "Module1", Version: "v1.0"}, Enabled: true, Valid: true},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kyma := &Kyma{
				Spec: tt.KymaSpec,
			}
			if got := kyma.GetAvailableModules(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAvailableModules() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_GetAvailableModules_When_ModuleInStatusOnly(t *testing.T) {
	tests := []struct {
		name       string
		KymaStatus KymaStatus
		want       []AvailableModule
	}{
		{
			name: "When Template exists, then the module is valid",
			KymaStatus: KymaStatus{
				Modules: []ModuleStatus{
					{
						Name:     "Module1",
						Channel:  "regular",
						Version:  "v1.0",
						Template: &TrackingObject{TypeMeta: apimetav1.TypeMeta{Kind: "ModuleTemplate"}},
					},
				},
			},
			want: []AvailableModule{
				{Module: Module{Name: "Module1", Channel: "regular", Version: "v1.0"}, Enabled: false, Valid: true},
			},
		},
		{
			name: "When Template not exists,then the module is invalid",
			KymaStatus: KymaStatus{
				Modules: []ModuleStatus{
					{
						Name:     "Module1",
						Channel:  "regular",
						Version:  "v1.0",
						Template: nil,
					},
				},
			},
			want: []AvailableModule{
				{Module: Module{Name: "Module1", Channel: "regular", Version: "v1.0"}, Enabled: false, Valid: false},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kyma := &Kyma{
				Status: tt.KymaStatus,
			}
			if got := kyma.GetAvailableModules(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAvailableModules() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_GetAvailableModules_When_ModuleExistsInSpecAndStatus(t *testing.T) {
	tests := []struct {
		name       string
		KymaSpec   KymaSpec
		KymaStatus KymaStatus
		want       []AvailableModule
	}{
		{
			name: "When Module have different version between Spec and Status, the output should be based on Spec",
			KymaSpec: KymaSpec{
				Modules: []Module{
					{Name: "Module1", Version: "v1.1"},
				},
			},
			KymaStatus: KymaStatus{
				Modules: []ModuleStatus{
					{
						Name:    "Module1",
						Version: "v1.0",
					},
				},
			},
			want: []AvailableModule{
				{Module: Module{Name: "Module1", Version: "v1.1"}, Enabled: true, Valid: true},
			},
		},
	}
	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			kyma := &Kyma{
				Spec:   test.KymaSpec,
				Status: test.KymaStatus,
			}
			if got := kyma.GetAvailableModules(); !reflect.DeepEqual(got, test.want) {
				t.Errorf("GetAvailableModules() = %v, want %v", got, test.want)
			}
		})
	}
}
