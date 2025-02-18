package remote //nolint:testpackage // testing package internals

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func TestReplaceWithVirtualKyma(t *testing.T) {
	t.Parallel()
	type testKyma struct {
		channel     string
		moduleNames []string
	}
	tests := []struct {
		name         string
		remoteKyma   testKyma
		kcpKyma      testKyma
		expectedKyma testKyma
	}{
		{
			"modules in kcp Kyma get replaced with modules in remote Kyma",
			testKyma{
				channel:     "regular",
				moduleNames: []string{"module1", "module3"},
			},
			testKyma{
				channel:     "regular",
				moduleNames: []string{"module2"},
			},
			testKyma{
				channel:     "regular",
				moduleNames: []string{"module1", "module3"},
			},
		},
		{
			"channel in kcp Kym gets replaced with channel in remote Kyma",
			testKyma{
				channel: "regular",
			},
			testKyma{
				channel: "fast",
			},
			testKyma{
				channel: "regular",
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			kcpKyma := createKyma(testCase.kcpKyma.channel, testCase.kcpKyma.moduleNames)
			remoteKyma := createKyma(testCase.remoteKyma.channel, testCase.remoteKyma.moduleNames)
			ReplaceSpec(kcpKyma, remoteKyma)
			assert.Equal(t, testCase.expectedKyma.channel, kcpKyma.Spec.Channel)
			var virtualModules []string
			for _, module := range kcpKyma.Spec.Modules {
				virtualModules = append(virtualModules, module.Name)
			}

			require.ElementsMatch(t, testCase.expectedKyma.moduleNames, virtualModules)
		})
	}
}

func createKyma(channel string, moduleNames []string) *v1beta2.Kyma {
	kyma := builder.NewKymaBuilder().
		WithChannel(channel).
		Build()

	modules := []v1beta2.Module{}
	for _, moduleName := range moduleNames {
		modules = append(modules, v1beta2.Module{
			Name:    moduleName,
			Channel: v1beta2.DefaultChannel,
			Managed: true,
		})
	}

	kyma.Spec.Modules = modules

	return kyma
}

func Test_syncStatus_AssignsRemoteNamespace(t *testing.T) {
	skrStatus := &v1beta2.KymaStatus{}
	kcpStatus := &v1beta2.KymaStatus{
		Modules: []v1beta2.ModuleStatus{
			{
				Name: "module-1",
				Template: &v1beta2.TrackingObject{
					PartialMeta: v1beta2.PartialMeta{
						Namespace: "kcp-system",
					},
				},
			},
			{
				Name: "module-2",
				Template: &v1beta2.TrackingObject{
					PartialMeta: v1beta2.PartialMeta{
						Namespace: "kcp-system",
					},
				},
			},
			{
				Name: "module-3",
			},
		},
	}

	syncStatus(kcpStatus, skrStatus)

	for _, module := range skrStatus.Modules {
		if module.Template == nil {
			continue
		}
		assert.Equal(t, shared.DefaultRemoteNamespace, module.Template.Namespace)
	}

	for _, module := range kcpStatus.Modules {
		if module.Template == nil {
			continue
		}
		assert.Equal(t, "kcp-system", module.Template.Namespace)
	}
}

func Test_syncStatus_RemovesManifestReference(t *testing.T) {
	skrStatus := &v1beta2.KymaStatus{}
	kcpStatus := &v1beta2.KymaStatus{
		Modules: []v1beta2.ModuleStatus{
			{
				Name: "module-1",
				Manifest: &v1beta2.TrackingObject{
					PartialMeta: v1beta2.PartialMeta{
						Namespace: "kcp-system",
					},
				},
			},
			{
				Name: "module-2",
				Manifest: &v1beta2.TrackingObject{
					PartialMeta: v1beta2.PartialMeta{
						Namespace: "kcp-system",
					},
				},
			},
			{
				Name: "module-3",
			},
		},
	}

	syncStatus(kcpStatus, skrStatus)

	for _, module := range skrStatus.Modules {
		if module.Manifest == nil {
			continue
		}
		assert.Nil(t, module.Manifest)
	}

	assert.NotNil(t, kcpStatus.Modules[0].Manifest)
	assert.NotNil(t, kcpStatus.Modules[1].Manifest)
}
