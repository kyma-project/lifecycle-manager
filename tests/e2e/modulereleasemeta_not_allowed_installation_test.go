package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ModuleReleaseMeta Not Allowed Installation", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	InitEmptyKymaBeforeAll(kyma)

	Context("Given SKR Cluster with ModuleTemplate and ModuleReleaseMeta", func() {
		It("When ModuleTemplate and ModuleReleaseMeta are applied in KCP cluster", func() {
			By("The the ModuleTemplate exists in the KCP Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(kcpClient, module, kyma).
				Should(Succeed())

			By("And the ModuleReleaseMeta exists on the KCP Cluster")
			Eventually(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, ControlPlaneNamespace, kcpClient).
				Should(Succeed())
		})

		It("When ModuleTemplate is marked as beta", func() {
			Eventually(SetModuleTemplateBetaLabel).
				WithContext(ctx).
				WithArguments(kcpClient, module, kyma, true).
				Should(Succeed())
		})

		It("When enabling the not allowed module", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, shared.DefaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())

			By("Then the module is in error state")
			Eventually(CheckModuleState).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, ControlPlaneNamespace, module.Name, shared.StateError).
				Should(Succeed())
		})

		It("When the beta label is removed from ModuleTemplate", func() {
			Eventually(SetModuleTemplateBetaLabel).
				WithContext(ctx).
				WithArguments(kcpClient, module, kyma, false).
				Should(Succeed())

			By("Then the module is in ready state")
			Eventually(CheckModuleState).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, ControlPlaneNamespace, module.Name, shared.StateReady).
				Should(Succeed())
		})

		It("When ModuleTemplate is marked as internal", func() {
			Eventually(SetModuleTemplateInternalLabel).
				WithContext(ctx).
				WithArguments(kcpClient, module, kyma, true).
				Should(Succeed())

			By("Then the module is in error state")
			Eventually(CheckModuleState).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, ControlPlaneNamespace, module.Name, shared.StateError).
				Should(Succeed())
		})
	})
})
