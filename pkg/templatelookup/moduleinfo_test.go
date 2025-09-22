package templatelookup_test

import (
	"errors"
	"strings"
	"testing"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
)

func Test_FetchModuleInfo_When_EmptySpecAndStatus(t *testing.T) {
	tests := []struct {
		name       string
		KymaSpec   v1beta2.KymaSpec
		KymaStatus v1beta2.KymaStatus
		want       []moduleInfo
	}{
		{
			name:       "When KymaSpec and KymaStatus are both empty, the output should be empty",
			KymaSpec:   v1beta2.KymaSpec{},
			KymaStatus: v1beta2.KymaStatus{},
			want:       []moduleInfo{}, // Expect empty result
		},
	}
	for ti := range tests {
		testcase := tests[ti]
		t.Run(testcase.name, func(t *testing.T) {
			kyma := &v1beta2.Kyma{
				Spec:   testcase.KymaSpec,
				Status: testcase.KymaStatus,
			}

			got := templatelookup.FetchModuleInfo(kyma)
			if len(got) != len(testcase.want) {
				t.Errorf("FetchModuleInfo() = %v, want %v", got, testcase.want)
			}
			for gi := range got {
				if !testcase.want[gi].Equals(got[gi]) {
					t.Errorf("FetchModuleInfo() = %v, want %v", got, testcase.want)
				}
			}
		})
	}
}

func Test_FetchModuleInfo_When_ModuleInSpecOnly(t *testing.T) {
	tests := []struct {
		name     string
		KymaSpec v1beta2.KymaSpec
		want     []moduleInfo
	}{
		{
			name: "When Channel \"none\" is used, then the module is invalid",
			KymaSpec: v1beta2.KymaSpec{
				Modules: []v1beta2.Module{
					{Name: "Module1", Channel: "none"},
				},
			},
			want: []moduleInfo{
				{
					Module:                  v1beta2.Module{Name: "Module1", Channel: "none"},
					Enabled:                 true,
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
			want: []moduleInfo{
				{
					Module:                  v1beta2.Module{Name: "Module1", Channel: "regular", Version: "v1.0"},
					Enabled:                 true,
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
			want: []moduleInfo{
				{Module: v1beta2.Module{Name: "Module1", Channel: "regular"}, Enabled: true},
			},
		},
		{
			name: "When Version is set, then the module is valid",
			KymaSpec: v1beta2.KymaSpec{
				Modules: []v1beta2.Module{
					{Name: "Module1", Version: "v1.0"},
				},
			},
			want: []moduleInfo{
				{Module: v1beta2.Module{Name: "Module1", Version: "v1.0"}, Enabled: true},
			},
		},
	}
	for ti := range tests {
		testcase := tests[ti]
		t.Run(testcase.name, func(t *testing.T) {
			kyma := &v1beta2.Kyma{
				Spec: testcase.KymaSpec,
			}

			got := templatelookup.FetchModuleInfo(kyma)
			if len(got) != len(testcase.want) {
				t.Errorf("FetchModuleInfo() = %v, want %v", got, testcase.want)
			}
			for gi := range got {
				if !testcase.want[gi].Equals(got[gi]) {
					t.Errorf("FetchModuleInfo() = %v, want %v", got, testcase.want)
				}
			}
		})
	}
}

func Test_FetchModuleInfo_When_ModuleInStatusOnly(t *testing.T) {
	tests := []struct {
		name       string
		KymaStatus v1beta2.KymaStatus
		want       []moduleInfo
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
			want: []moduleInfo{
				{Module: v1beta2.Module{Name: "Module1", Channel: "regular", Version: "v1.0"}, Enabled: false},
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
			want: []moduleInfo{
				{
					Module:                  v1beta2.Module{Name: "Module1", Channel: "regular", Version: "v1.0"},
					Enabled:                 false,
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

			got := templatelookup.FetchModuleInfo(kyma)
			if len(got) != len(testcase.want) {
				t.Errorf("FetchModuleInfo() = %v, want %v", got, testcase.want)
			}
			for gi := range got {
				if !testcase.want[gi].Equals(got[gi]) {
					t.Errorf("FetchModuleInfo() = %v, want %v", got, testcase.want)
				}
			}
		})
	}
}

func Test_FetchModuleInfo_When_ModuleExistsInSpecAndStatus(t *testing.T) {
	tests := []struct {
		name       string
		KymaSpec   v1beta2.KymaSpec
		KymaStatus v1beta2.KymaStatus
		want       []moduleInfo
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
			want: []moduleInfo{
				{Module: v1beta2.Module{Name: "Module1", Version: "v1.1"}, Enabled: true},
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
			got := templatelookup.FetchModuleInfo(kyma)
			if len(got) != len(testcase.want) {
				t.Errorf("FetchModuleInfo() = %v, want %v", got, testcase.want)
			}
			for gi := range got {
				if !testcase.want[gi].Equals(got[gi]) {
					t.Errorf("FetchModuleInfo() = %v, want %v", got, testcase.want)
				}
			}
		})
	}
}

type moduleInfo struct {
	Module                  v1beta2.Module
	Enabled                 bool
	ValidationErrorContains string
	ExpectedError           error
}

func (m moduleInfo) Equals(other templatelookup.ModuleInfo) bool {
	if m.Module.Name != other.Name {
		return false
	}
	if m.Module.Channel != other.Channel {
		return false
	}
	if m.Module.Version != other.Version {
		return false
	}
	if m.Enabled != other.Enabled {
		return false
	}
	if m.ExpectedError != nil && other.ValidationError == nil {
		return false
	}
	if m.ExpectedError == nil && other.ValidationError != nil {
		return false
	}
	if m.ExpectedError != nil && other.ValidationError != nil {
		if !errors.Is(other.ValidationError, m.ExpectedError) {
			return false
		}
		if !strings.Contains(other.ValidationError.Error(), m.ValidationErrorContains) {
			return false
		}
	}
	return true
}
