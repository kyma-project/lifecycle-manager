package v1beta2_test

import (
	"strings"
	"testing"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func Test_GetVersion(t *testing.T) {
	const testVersion = "1.0.1"
	const otherVersion = "0.0.1"
	tests := []struct {
		name            string
		m               *v1beta2.ModuleTemplate
		expectedVersion string
		expectedErr     string
	}{
		{
			name: "Test GetVersion() by annotation (legacy)",
			m: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{
						shared.ModuleVersionAnnotation: testVersion,
					},
				},
			},
			expectedVersion: testVersion,
		},
		{
			name: "Test GetVersion() by explicit version in Spec",
			m: &v1beta2.ModuleTemplate{
				Spec: v1beta2.ModuleTemplateSpec{
					Version: testVersion,
				},
			},
			expectedVersion: testVersion,
		},
		{
			name: "Test GetVersion() with both version in Spec and annotation",
			m: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{
						shared.ModuleVersionAnnotation: otherVersion,
					},
				},
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
			actualVersion, err := testCase.m.GetVersion()
			if err != nil {
				if actualVersion != nil {
					t.Errorf("GetVersion(): Returned version should be nil when error is not nil")
				}
				if testCase.expectedErr == "" {
					t.Errorf("GetVersion(): Unexpected error: %v", err)
				}
				if !strings.Contains(err.Error(), testCase.expectedErr) {
					t.Errorf("GetVersion(): Actual error = %v, expected error: %v", err, testCase.expectedErr)
				}
				return
			}

			if actualVersion == nil {
				t.Errorf("GetVersion(): Returned version should not be nil when error is nil")
			}

			if testCase.expectedVersion == "" {
				t.Errorf("GetVersion(): Expected version is empty but non-nil version is returned")
			}

			if actualVersion != nil && actualVersion.String() != testCase.expectedVersion {
				t.Errorf("GetVersion(): actual version = %v, expected version: %v", actualVersion.String(),
					testCase.expectedVersion)
			}
		})
	}
}
