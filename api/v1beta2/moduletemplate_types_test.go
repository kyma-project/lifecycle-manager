package v1beta2_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

//nolint:dupl  // similar but not duplicate
func Test_GetVersion(t *testing.T) {
	tests := []struct {
		name            string
		m               *v1beta2.ModuleTemplate
		expectedVersion string
	}{
		{
			name: "Test GetVersion() by annotation (legacy)",
			m: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{
						shared.ModuleVersionAnnotation: "1.0.0-annotated",
					},
				},
				Spec: v1beta2.ModuleTemplateSpec{},
			},
			expectedVersion: "1.0.0-annotated",
		},
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
		{
			name: "Test GetVersion() spec.moduleName has priority over annotation",
			m: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{
						shared.ModuleVersionAnnotation: "1.0.0-annotated",
					},
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
