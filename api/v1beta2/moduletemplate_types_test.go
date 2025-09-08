package v1beta2_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func Test_GetSemanticVersion(t *testing.T) {
	const testVersion = "1.0.1"
	tests := []struct {
		name            string
		m               *v1beta2.ModuleTemplate
		expectedVersion string
		expectedErr     string
	}{
		{
			name: "Test GetSemanticVersion() by explicit version in Spec",
			m: &v1beta2.ModuleTemplate{
				Spec: v1beta2.ModuleTemplateSpec{
					Version: testVersion,
				},
			},
			expectedVersion: testVersion,
		},
		{
			name: "Test GetVersion without any version info",
			m: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Spec: v1beta2.ModuleTemplateSpec{},
			},
			expectedErr: v1beta2.ErrInvalidVersion.Error(),
		},
		{
			name: "Test GetVersion with invalid version",
			m: &v1beta2.ModuleTemplate{
				Spec: v1beta2.ModuleTemplateSpec{
					Version: "invalid",
				},
			},
			expectedErr: "Invalid Semantic Version",
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			actualVersion, err := testCase.m.GetSemanticVersion()
			if err != nil {
				if actualVersion != nil {
					t.Errorf("GetSemanticVersion(): Returned version should be nil when error is not nil")
				}
				if testCase.expectedErr == "" {
					t.Errorf("GetSemanticVersion(): Unexpected error: %v", err)
				}
				if !strings.Contains(err.Error(), testCase.expectedErr) {
					t.Errorf("GetSemanticVersion(): Actual error = %v, expected error: %v", err, testCase.expectedErr)
				}
				return
			}

			if actualVersion == nil {
				t.Errorf("GetSemanticVersion(): Returned version should not be nil when error is nil")
			}

			if testCase.expectedVersion == "" {
				t.Errorf("GetSemanticVersion(): Expected version is empty but non-nil version is returned")
			}

			if actualVersion != nil && actualVersion.String() != testCase.expectedVersion {
				t.Errorf("GetSemanticVersion(): actual version = %v, expected version: %v", actualVersion.String(),
					testCase.expectedVersion)
			}
		})
	}
}

//nolint:dupl  // similar but not duplicate
func Test_GetVersion(t *testing.T) {
	tests := []struct {
		name            string
		m               *v1beta2.ModuleTemplate
		expectedVersion string
	}{
		{
			name: "Test GetVersion() by spec.version",
			m: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Spec: v1beta2.ModuleTemplateSpec{
					Version: "2.0.0-spec",
				},
			},
			expectedVersion: "2.0.0-spec",
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			actualVersion := testCase.m.GetVersion()
			assert.Equal(t, testCase.expectedVersion, actualVersion)
		})
	}
}

//nolint:dupl  // similar but not duplicate
func Test_GetModuleName(t *testing.T) {
	tests := []struct {
		name         string
		m            *v1beta2.ModuleTemplate
		expectedName string
	}{
		{
			name: "Test GetManagerName() by label",
			m: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: map[string]string{
						shared.ModuleName: "labelled-module",
					},
				},
				Spec: v1beta2.ModuleTemplateSpec{},
			},
			expectedName: "labelled-module",
		},
		{
			name: "Test GetManagerName() by spec.moduleName",
			m: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: map[string]string{},
				},
				Spec: v1beta2.ModuleTemplateSpec{
					ModuleName: "spec-module",
				},
			},
			expectedName: "spec-module",
		},
		{
			name: "Test GetManagerName() spec.moduleName has priority over label",
			m: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: map[string]string{
						shared.ModuleName: "labelled-module",
					},
				},
				Spec: v1beta2.ModuleTemplateSpec{
					ModuleName: "spec-module",
				},
			},
			expectedName: "spec-module",
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			actualName := testCase.m.GetModuleName()
			assert.Equal(t, testCase.expectedName, actualName)
		})
	}
}

func TestGenerateDescriptorCacheKey(t *testing.T) {
	testCases := []struct {
		name        string
		template    *v1beta2.ModuleTemplate
		want        string
		expectError bool
	}{
		{
			name: "Module Version is not nil and valid semver",
			template: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:       "name",
					Generation: 1,
				},
				Spec: v1beta2.ModuleTemplateSpec{
					Version: "1.0.0",
				},
			},
			want:        "name:1:1.0.0",
			expectError: false,
		},
		{
			name: "Module Version is not nil but invalid semver",
			template: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:       "name",
					Generation: 1,
				},
				Spec: v1beta2.ModuleTemplateSpec{
					Version: "not-semver",
				},
			},
			want:        "",
			expectError: true,
		},
		{
			name: "Module Version is not nil but module version is empty",
			template: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:       "name",
					Generation: 2,
				},
				Spec: v1beta2.ModuleTemplateSpec{
					Version: "",
				},
			},
			want:        "",
			expectError: true,
		},
		{
			name: "Module Version is nil",
			template: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:       "name",
					Generation: 2,
				},
			},
			want:        "",
			expectError: true,
		},
	}

	for i := range testCases {
		t.Run(testCases[i].name, func(t *testing.T) {
			got, err := testCases[i].template.GenerateDescriptorKey()
			if testCases[i].expectError {
				assert.Error(t, err, "expected error but got none")
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.Equalf(t, testCases[i].want, got,
					"GenerateDescriptorKey() = %v, want %v", got, testCases[i].want)
			}
		})
	}
}
