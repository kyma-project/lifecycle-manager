package templatelookup_test

import (
	"errors"
	"strings"
	"testing"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
)

func Test_FetchModuleStatusInfo_When_EmptySpecAndStatus(t *testing.T) {
	tests := []struct {
		name       string
		KymaSpec   v1beta2.KymaSpec
		KymaStatus v1beta2.KymaStatus
		want       []moduleStatusInfo
	}{
		{
			name:       "When KymaSpec and KymaStatus are both empty, the output should be empty",
			KymaSpec:   v1beta2.KymaSpec{},
			KymaStatus: v1beta2.KymaStatus{},
			want:       []moduleStatusInfo{}, // Expect empty result
		},
	}
	for ti := range tests {
		testcase := tests[ti]
		t.Run(testcase.name, func(t *testing.T) {
			kyma := &v1beta2.Kyma{
				Spec:   testcase.KymaSpec,
				Status: testcase.KymaStatus,
			}

			got := templatelookup.FetchModuleStatusInfo(kyma)
			if len(got) != len(testcase.want) {
				t.Errorf("FetchModuleStatusInfo() = %v, want %v", got, testcase.want)
			}
			for gi := range got {
				if !testcase.want[gi].Equals(got[gi]) {
					t.Errorf("FetchModuleStatusInfo() = %v, want %v", got, testcase.want)
				}
			}
		})
	}
}

func Test_FetchModuleStatusInfo_When_ModuleInSpecOnly(t *testing.T) {
	tests := []struct {
		name     string
		KymaSpec v1beta2.KymaSpec
		want     []moduleStatusInfo
	}{
		{
			name: "When Channel \"none\" is used, then the module is invalid",
			KymaSpec: v1beta2.KymaSpec{
				Modules: []v1beta2.Module{
					{Name: "Module1", Channel: "none"},
				},
			},
			want: []moduleStatusInfo{
				{
					Module:                  v1beta2.Module{Name: "Module1", Channel: "none"},
					IsEnabled:               true,
					ValidationErrorContains: "Channel \"none\" is not allowed",
					ExpectedError:           templatelookup.ErrInvalidModuleInSpec,
				},
			},
		},
		{
			name: "When Channel and Version are both set, then the module is invalid",
			KymaSpec: v1beta2.KymaSpec{
				Modules: []v1beta2.Module{
					{Name: "Module1", Channel: "regular", Version: "v1.0"},
				},
			},
			want: []moduleStatusInfo{
				{
					Module:                  v1beta2.Module{Name: "Module1", Channel: "regular", Version: "v1.0"},
					IsEnabled:               true,
					ValidationErrorContains: "Version and channel are mutually exclusive options",
					ExpectedError:           templatelookup.ErrInvalidModuleInSpec,
				},
			},
		},
		{
			name: "When Channel is set, then the module is valid",
			KymaSpec: v1beta2.KymaSpec{
				Modules: []v1beta2.Module{
					{Name: "Module1", Channel: "regular"},
				},
			},
			want: []moduleStatusInfo{
				{Module: v1beta2.Module{Name: "Module1", Channel: "regular"}, IsEnabled: true},
			},
		},
		{
			name: "When Version is set, then the module is valid",
			KymaSpec: v1beta2.KymaSpec{
				Modules: []v1beta2.Module{
					{Name: "Module1", Version: "v1.0"},
				},
			},
			want: []moduleStatusInfo{
				{Module: v1beta2.Module{Name: "Module1", Version: "v1.0"}, IsEnabled: true},
			},
		},
	}
	for ti := range tests {
		testcase := tests[ti]
		t.Run(testcase.name, func(t *testing.T) {
			kyma := &v1beta2.Kyma{
				Spec: testcase.KymaSpec,
			}

			got := templatelookup.FetchModuleStatusInfo(kyma)
			if len(got) != len(testcase.want) {
				t.Errorf("FetchModuleStatusInfo() = %v, want %v", got, testcase.want)
			}
			for gi := range got {
				if !testcase.want[gi].Equals(got[gi]) {
					t.Errorf("FetchModuleStatusInfo() = %v, want %v", got, testcase.want)
				}
			}
		})
	}
}

func Test_FetchModuleStatusInfo_When_ModuleInStatusOnly(t *testing.T) {
	tests := []struct {
		name       string
		KymaStatus v1beta2.KymaStatus
		want       []moduleStatusInfo
	}{
		{
			name: "When Template exists, then the module is valid",
			KymaStatus: v1beta2.KymaStatus{
				Modules: []v1beta2.ModuleStatus{
					{
						Name:     "Module1",
						Channel:  "regular",
						Version:  "v1.0",
						Template: &v1beta2.TrackingObject{TypeMeta: apimetav1.TypeMeta{Kind: "ModuleTemplate"}},
					},
				},
			},
			want: []moduleStatusInfo{
				{Module: v1beta2.Module{Name: "Module1", Channel: "regular", Version: "v1.0"}, IsEnabled: false},
			},
		},
		{
			name: "When Template not exists,then the module is invalid",
			KymaStatus: v1beta2.KymaStatus{
				Modules: []v1beta2.ModuleStatus{
					{
						Name:     "Module1",
						Channel:  "regular",
						Version:  "v1.0",
						Template: nil,
					},
				},
			},
			want: []moduleStatusInfo{
				{
					Module:                  v1beta2.Module{Name: "Module1", Channel: "regular", Version: "v1.0"},
					IsEnabled:               false,
					ValidationErrorContains: "ModuleTemplate reference is missing",
					ExpectedError:           templatelookup.ErrInvalidModuleInStatus,
				},
			},
		},
	}
	for ti := range tests {
		testcase := tests[ti]
		t.Run(testcase.name, func(t *testing.T) {
			kyma := &v1beta2.Kyma{
				Status: testcase.KymaStatus,
			}

			got := templatelookup.FetchModuleStatusInfo(kyma)
			if len(got) != len(testcase.want) {
				t.Errorf("FetchModuleStatusInfo() = %v, want %v", got, testcase.want)
			}
			for gi := range got {
				if !testcase.want[gi].Equals(got[gi]) {
					t.Errorf("FetchModuleStatusInfo() = %v, want %v", got, testcase.want)
				}
			}
		})
	}
}

func Test_FetchModuleStatusInfo_When_ModuleExistsInSpecAndStatus(t *testing.T) {
	tests := []struct {
		name       string
		KymaSpec   v1beta2.KymaSpec
		KymaStatus v1beta2.KymaStatus
		want       []moduleStatusInfo
	}{
		{
			name: "When Module have different version between Spec and Status, the output should be based on Spec",
			KymaSpec: v1beta2.KymaSpec{
				Modules: []v1beta2.Module{
					{Name: "Module1", Version: "v1.1"},
				},
			},
			KymaStatus: v1beta2.KymaStatus{
				Modules: []v1beta2.ModuleStatus{
					{
						Name:    "Module1",
						Version: "v1.0",
					},
				},
			},
			want: []moduleStatusInfo{
				{Module: v1beta2.Module{Name: "Module1", Version: "v1.1"}, IsEnabled: true},
			},
		},
	}
	for ti := range tests {
		testcase := tests[ti]
		t.Run(testcase.name, func(t *testing.T) {
			kyma := &v1beta2.Kyma{
				Spec:   testcase.KymaSpec,
				Status: testcase.KymaStatus,
			}
			got := templatelookup.FetchModuleStatusInfo(kyma)
			if len(got) != len(testcase.want) {
				t.Errorf("FetchModuleStatusInfo() = %v, want %v", got, testcase.want)
			}
			for gi := range got {
				if !testcase.want[gi].Equals(got[gi]) {
					t.Errorf("FetchModuleStatusInfo() = %v, want %v", got, testcase.want)
				}
			}
		})
	}
}

type moduleStatusInfo struct {
	Module                  v1beta2.Module
	IsEnabled               bool
	ValidationErrorContains string
	ExpectedError           error
}

func (amd moduleStatusInfo) Equals(other templatelookup.ModuleStatusInfo) bool {
	if amd.Module.Name != other.Name {
		return false
	}
	if amd.Module.Channel != other.Channel {
		return false
	}
	if amd.Module.Version != other.Version {
		return false
	}
	if amd.IsEnabled != other.IsEnabled {
		return false
	}
	if amd.ExpectedError != nil && other.ValidationError == nil {
		return false
	}
	if amd.ExpectedError == nil && other.ValidationError != nil {
		return false
	}
	if amd.ExpectedError != nil && other.ValidationError != nil {
		if !errors.Is(other.ValidationError, amd.ExpectedError) {
			return false
		}
		if !strings.Contains(other.ValidationError.Error(), amd.ValidationErrorContains) {
			return false
		}
	}
	return true
}
