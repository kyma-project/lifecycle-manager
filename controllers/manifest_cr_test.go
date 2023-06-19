package controllers_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Update Manifest CR", Ordered, func() {
	kyma := NewTestKyma("kyma-test-update")

	kyma.Spec.Modules = append(
		kyma.Spec.Modules, v1beta2.Module{
			ControllerName: "manifest",
			Name:           "skr-module-update",
			Channel:        v1beta2.DefaultChannel,
		})

	RegisterDefaultLifecycleForKyma(kyma)

	It("Manifest CR should be updated after module template changed", func() {
		By("CR created")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(ManifestExists, Timeout, Interval).WithArguments(
				ctx, kyma, activeModule, controlPlaneClient).Should(Succeed())
		}

		By("reacting to a change of its Modules when they are set to ready")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(UpdateModuleState(ctx, kyma, activeModule, v1beta2.StateReady),
				Timeout, Interval).Should(Succeed())
		}

		By("Kyma CR should be in Ready state")
		Eventually(GetKymaState, Timeout, Interval).
			WithArguments(kyma.GetName()).
			Should(BeEquivalentTo(string(v1beta2.StateReady)))

		By("Update Module Template spec.data.spec field")
		valueUpdated := "valueUpdated"
		Eventually(updateKCPModuleTemplateSpecData(kyma.Name, valueUpdated), Timeout, Interval).Should(Succeed())

		By("CR updated with new value in spec.resource.spec")
		Eventually(expectManifestSpecDataEquals(kyma.Name, valueUpdated), Timeout, Interval).Should(Succeed())
	})
})
