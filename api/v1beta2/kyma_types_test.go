package v1beta2_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func Test_GetModuleStatus(t *testing.T) {
	tests := []struct {
		name          string
		kymaStatus    *v1beta2.KymaStatus
		moduleName    string
		expectSuccess bool
	}{
		{
			name: "Test GetModuleStatus() with existing module",
			kymaStatus: &v1beta2.KymaStatus{
				Modules: []v1beta2.ModuleStatus{
					{
						Name: "module1",
					},
				},
			},
			moduleName:    "module1",
			expectSuccess: true,
		},
		{
			name: "Test GetModuleStatus() with non-existing module",
			kymaStatus: &v1beta2.KymaStatus{
				Modules: []v1beta2.ModuleStatus{
					{
						Name: "module1",
					},
				},
			},
			moduleName:    "module2",
			expectSuccess: false,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			moduleStatus := testCase.kymaStatus.GetModuleStatus(testCase.moduleName)
			if testCase.expectSuccess {
				assert.NotNil(t, moduleStatus)
				assert.Equal(t, testCase.moduleName, moduleStatus.Name)
			} else {
				assert.Nil(t, moduleStatus)
			}
		})
	}
}

func Test_GetGlobalAccountID(t *testing.T) {
	tests := []struct {
		name          string
		labels        map[string]string
		expectSuccess bool
	}{
		{
			name: "Test GetGlobalAccountID() with existing label",
			labels: map[string]string{
				shared.GlobalAccountIDLabel: "1234",
			},
			expectSuccess: true,
		},
		{
			name:          "Test GetGlobalAccountID() with non-existing label",
			labels:        map[string]string{},
			expectSuccess: false,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			kyma := &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: testCase.labels,
					Name:   "test-kyma",
				},
			}

			globalAccountID := kyma.GetGlobalAccount()
			if testCase.expectSuccess {
				assert.Equal(t, testCase.labels[shared.GlobalAccountIDLabel], globalAccountID)
			} else {
				assert.Empty(t, globalAccountID)
			}
		})
	}
}

func Test_GetRegion(t *testing.T) {
	tests := []struct {
		name          string
		labels        map[string]string
		expectSuccess bool
	}{
		{
			name: "Test GetRegion() with existing label",
			labels: map[string]string{
				shared.RegionLabel: "eu-central-1",
			},
			expectSuccess: true,
		},
		{
			name:          "Test GetRegion() with non-existing label",
			labels:        map[string]string{},
			expectSuccess: false,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			kyma := &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: testCase.labels,
					Name:   "test-kyma",
				},
			}

			region := kyma.GetRegion()
			if testCase.expectSuccess {
				assert.Equal(t, testCase.labels[shared.RegionLabel], region)
			} else {
				assert.Empty(t, region)
			}
		})
	}
}

func Test_GetPlatformRegion(t *testing.T) {
	tests := []struct {
		name          string
		labels        map[string]string
		expectSuccess bool
	}{
		{
			name: "Test GetPlatformRegion() with existing label",
			labels: map[string]string{
				shared.PlatformRegionLabel: "cf-us10-staging",
			},
			expectSuccess: true,
		},
		{
			name:          "Test GetPlatformRegion() with non-existing label",
			labels:        map[string]string{},
			expectSuccess: false,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			kyma := &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: testCase.labels,
					Name:   "test-kyma",
				},
			}

			platformRegion := kyma.GetPlatformRegion()
			if testCase.expectSuccess {
				assert.Equal(t, testCase.labels[shared.PlatformRegionLabel], platformRegion)
			} else {
				assert.Empty(t, platformRegion)
			}
		})
	}
}

func Test_GetPlan(t *testing.T) {
	tests := []struct {
		name          string
		labels        map[string]string
		expectSuccess bool
	}{
		{
			name: "Test GetPlan() with existing label",
			labels: map[string]string{
				shared.PlanLabel: "aws",
			},
			expectSuccess: true,
		},
		{
			name:          "Test GetPlan() with non-existing label",
			labels:        map[string]string{},
			expectSuccess: false,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			kyma := &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: testCase.labels,
					Name:   "test-kyma",
				},
			}

			plan := kyma.GetPlan()
			if testCase.expectSuccess {
				assert.Equal(t, testCase.labels[shared.PlanLabel], plan)
			} else {
				assert.Empty(t, plan)
			}
		})
	}
}

func Test_GetRuntimeID(t *testing.T) {
	tests := []struct {
		name          string
		labels        map[string]string
		expectSuccess bool
	}{
		{
			name: "Test GetRuntimeID() with existing label",
			labels: map[string]string{
				shared.RuntimeIDLabel: "test123",
			},
			expectSuccess: true,
		},
		{
			name:          "Test GetRuntimeID() with non-existing label",
			labels:        map[string]string{},
			expectSuccess: false,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			kyma := &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Labels: testCase.labels,
					Name:   "test-kyma",
				},
			}

			id := kyma.GetRuntimeID()
			if testCase.expectSuccess {
				assert.Equal(t, testCase.labels[shared.RuntimeIDLabel], id)
			} else {
				assert.Empty(t, id)
			}
		})
	}
}
