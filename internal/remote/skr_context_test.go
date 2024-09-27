package remote_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

func TestReplaceWithVirtualKyma(t *testing.T) {
	t.Parallel()
	type testKyma struct {
		channel string
		modules []string
	}
	tests := []struct {
		name         string
		remoteKyma   testKyma
		kcpKyma      testKyma
		expectedKyma testKyma
	}{
		{
			"module in kcp Kyma gets replaced with remote Kyma",
			testKyma{
				channel: "regular",
				modules: []string{"module1"},
			},
			testKyma{
				channel: "regular",
				modules: []string{"module2"},
			},
			testKyma{
				channel: "regular",
				modules: []string{"module1"},
			},
		},
		{
			"channel updated in remoteKyma be the final channel",
			testKyma{
				channel: "regular",
				modules: []string{"module1"},
			},
			testKyma{
				channel: "fast",
				modules: []string{"module1"},
			},
			testKyma{
				channel: "regular",
				modules: []string{"module1"},
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			kcpKyma := createKyma(testCase.kcpKyma.channel, testCase.kcpKyma.modules)
			remoteKyma := createKyma(testCase.remoteKyma.channel, testCase.remoteKyma.modules)
			remote.ReplaceSpec(kcpKyma, remoteKyma)
			assert.Equal(t, testCase.expectedKyma.channel, kcpKyma.Spec.Channel)
			var virtualModules []string
			for _, module := range kcpKyma.Spec.Modules {
				virtualModules = append(virtualModules, module.Name)
			}

			require.ElementsMatch(t, testCase.expectedKyma.modules, virtualModules)
		})
	}
}

func createKyma(channel string, modules []string) *v1beta2.Kyma {
	kcpKyma := testutils.NewTestKyma("test-kyma")

	kcpKyma.Spec.Channel = channel
	for _, module := range modules {
		kcpKyma.Spec.Modules = append(
			kcpKyma.Spec.Modules, v1beta2.Module{
				Name:    module,
				Channel: v1beta2.DefaultChannel,
				Managed: true,
			})
	}
	return kcpKyma
}
