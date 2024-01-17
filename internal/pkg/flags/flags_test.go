package flags_test

import (
	"fmt"
	"testing"

	. "github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
)

func Test_ConstantFlags(t *testing.T) {
	t.Parallel()
	tests := []struct {
		constName     string
		constValue    string
		expectedValue string
	}{
		{
			constName:     "DefaultRemoteSyncNamespace",
			constValue:    DefaultRemoteSyncNamespace,
			expectedValue: "kyma-system",
		},
		{
			constName:     "DefaultIstioGatewayName",
			constValue:    DefaultIstioGatewayName,
			expectedValue: "klm-watcher-gateway",
		},
		{
			constName:     "DefaultIstioGatewayNamespace",
			constValue:    DefaultIstioGatewayNamespace,
			expectedValue: "kcp-system",
		},
		{
			constName:     "DefaultIstioNamespace",
			constValue:    DefaultIstioNamespace,
			expectedValue: "istio-system",
		},
		{
			constName:     "DefaultCaCertName",
			constValue:    DefaultCaCertName,
			expectedValue: "klm-watcher-serving-cert",
		},
	}
	for _, testcase := range tests {
		testcase := testcase
		testName := fmt.Sprintf("const %s has correct value", testcase.constName)
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			if testcase.constValue != testcase.expectedValue {
				t.Errorf("const %s value is incorrect: expected = %s, got = %s",
					testcase.constName, testcase.expectedValue, testcase.constValue)
			}
		})
	}
}
