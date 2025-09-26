package v1beta2_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

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
		{
			name: "Test GetVersion() returns empty string when no version in spec",
			m: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Spec: v1beta2.ModuleTemplateSpec{},
			},
			expectedVersion: "",
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			actualVersion := testCase.m.GetVersion()
			assert.Equal(t, testCase.expectedVersion, actualVersion)
		})
	}
}

func Test_GetModuleName(t *testing.T) {
	tests := []struct {
		name         string
		m            *v1beta2.ModuleTemplate
		expectedName string
	}{
		{
			name: "Test GetModuleName() by spec.moduleName",
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
			name: "Test GetModuleName() returns empty string when no module name in spec",
			m: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: map[string]string{},
				},
				Spec: v1beta2.ModuleTemplateSpec{},
			},
			expectedName: "",
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			actualName := testCase.m.GetModuleName()
			assert.Equal(t, testCase.expectedName, actualName)
		})
	}
}

func Test_IsBeta(t *testing.T) {
	tests := []struct {
		name         string
		m            *v1beta2.ModuleTemplate
		expectedBeta bool
	}{
		{
			name: "Test IsBeta() returns true when beta label is enabled",
			m: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: map[string]string{
						shared.BetaLabel: shared.EnableLabelValue,
					},
				},
			},
			expectedBeta: true,
		},
		{
			name: "Test IsBeta() returns false when beta label is disabled",
			m: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: map[string]string{
						shared.BetaLabel: "false",
					},
				},
			},
			expectedBeta: false,
		},
		{
			name: "Test IsBeta() returns true when beta label is enabled with mixed case",
			m: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: map[string]string{
						shared.BetaLabel: "TRUE",
					},
				},
			},
			expectedBeta: true,
		},
		{
			name: "Test IsBeta() returns false when no labels",
			m: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{},
			},
			expectedBeta: false,
		},
		{
			name: "Test IsBeta() returns false when beta label missing",
			m: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: map[string]string{
						"other-label": "value",
					},
				},
			},
			expectedBeta: false,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			actualBeta := testCase.m.IsBeta()
			assert.Equal(t, testCase.expectedBeta, actualBeta)
		})
	}
}

func Test_IsInternal(t *testing.T) {
	tests := []struct {
		name             string
		m                *v1beta2.ModuleTemplate
		expectedInternal bool
	}{
		{
			name: "Test IsInternal() returns true when internal label is enabled",
			m: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: map[string]string{
						shared.InternalLabel: shared.EnableLabelValue,
					},
				},
			},
			expectedInternal: true,
		},
		{
			name: "Test IsInternal() returns false when internal label is disabled",
			m: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: map[string]string{
						shared.InternalLabel: "false",
					},
				},
			},
			expectedInternal: false,
		},
		{
			name: "Test IsInternal() returns true when internal label is enabled with mixed case",
			m: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: map[string]string{
						shared.InternalLabel: "TRUE",
					},
				},
			},
			expectedInternal: true,
		},
		{
			name: "Test IsInternal() returns false when no labels",
			m: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{},
			},
			expectedInternal: false,
		},
		{
			name: "Test IsInternal() returns false when internal label missing",
			m: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: map[string]string{
						"other-label": "value",
					},
				},
			},
			expectedInternal: false,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			actualInternal := testCase.m.IsInternal()
			assert.Equal(t, testCase.expectedInternal, actualInternal)
		})
	}
}
