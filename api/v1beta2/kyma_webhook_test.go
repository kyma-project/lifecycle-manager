package v1beta2_test

import (
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

func Test_validateKymaModule(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		enabledModules []string
		wantErr        bool
	}{
		{
			"modules without duplicate",
			[]string{"module1", "module2"},
			false,
		},
		{
			"modules with duplicate",
			[]string{"module1", "module1"},
			true,
		},
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			kyma := testutils.NewTestKyma("test-kyma")
			for _, moduleName := range testCase.enabledModules {
				module := v1beta2.Module{
					Name: moduleName,
				}
				kyma.Spec.Modules = append(kyma.Spec.Modules, module)
			}
			if err := v1beta2.ValidateKymaModule(kyma); (err != nil) != testCase.wantErr {
				t.Errorf("ValidateKymaModule() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}
