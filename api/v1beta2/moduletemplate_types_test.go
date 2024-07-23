package v1beta2

import (
	"strings"
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_GetVersion(t *testing.T) {
	const testVersion = "1.0.1"
	const otherVersion = "0.0.1"
	tests := []struct {
		name            string
		m               *ModuleTemplate
		expectedVersion string
		expectedErr     string
	}{
		{
			name: "Test GetVersion() by annotation (legacy)",
			m: &ModuleTemplate{
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
			m: &ModuleTemplate{
				Spec: ModuleTemplateSpec{
					Version: testVersion,
				},
			},
			expectedVersion: testVersion,
		},
		{
			name: "Test GetVersion() with both version in Spec and annotation",
			m: &ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{
						shared.ModuleVersionAnnotation: otherVersion,
					},
				},
				Spec: ModuleTemplateSpec{
					Version: testVersion,
				},
			},
			expectedVersion: testVersion,
		},
		{
			name: "Test GetVersion without any version info",
			m: &ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Spec: ModuleTemplateSpec{},
			},
			expectedErr: ErrInvalidVersion.Error(),
		},
		{
			name: "Test GetVersion with invalid version",
			m: &ModuleTemplate{
				Spec: ModuleTemplateSpec{
					Version: "invalid",
				},
			},
			expectedErr: "Invalid Semantic Version",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			actualVersion, err := tt.m.GetVersion()
			if err != nil {
				if actualVersion != nil {
					t.Errorf("GetVersion(): Returned version should be nil when error is not nil")
				}
				if tt.expectedErr == "" {
					t.Errorf("GetVersion(): Unexpected error: %v", err)
				}
				if !strings.Contains(err.Error(), tt.expectedErr) {
					t.Errorf("GetVersion(): Actual error = %v, expected error: %v", err, tt.expectedErr)
				}
				return
			}

			if actualVersion == nil {
				t.Errorf("GetVersion(): Returned version should not be nil when error is nil")
			}

			if tt.expectedVersion == "" {
				t.Errorf("GetVersion(): Expected version is empty but non-nil version is returned")
			}

			if actualVersion.String() != tt.expectedVersion {
				t.Errorf("GetVersion(): actual version = %v, expected version: %v", actualVersion.String(), tt.expectedVersion)
			}
		})
	}
}
