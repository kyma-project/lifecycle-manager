package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ModuleReleaseMeta Watch Trigger", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(RemoteNamespace)
	moduleReleaseMetaNamespace := "kcp-system"
	moduleReleaseMetaName := "template-operator"

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given kyma deployed in KCP", func() {
		It("When enabling Template Operator", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Succeed())
		})

		It("Then KCP Kyma CR is in \"Ready\" State", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
			Consistently(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})

		It("When ModuleReleaseMeta channels get updated with invalid version", func() {
			Eventually(UpdateAllModuleReleaseMetaChannelVersions).
				WithContext(ctx).
				WithArguments(kcpClient, moduleReleaseMetaNamespace, moduleReleaseMetaName, "1.2.3").
				Should(Succeed())
		})
		It("Then KCP Kyma CR should be requeued and gets into \"Error\" State", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateError).
				Should(Succeed())
			Consistently(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateError).
				Should(Succeed())
			By("And the Module Status is in \"Error\" State", func() {
				Eventually(CheckModuleState).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name, shared.StateError).
					Should(Succeed())
			})
			By("And the Module Status has correct error message", func() {
				Eventually(ModuleMessageInKymaStatusIsCorrect).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name,
						"failed to get module template: "+
							"ModuleTemplate.operator.kyma-project.io \"template-operator-1.2.3\" not found").
					Should(Succeed())
			})
		})

		It("When ModuleReleaseMeta channels get updated with valid version", func() {
			Eventually(UpdateAllModuleReleaseMetaChannelVersions).
				WithContext(ctx).
				WithArguments(kcpClient, moduleReleaseMetaNamespace, moduleReleaseMetaName, ModuleVersionToBeUsed).
				Should(Succeed())
		})
		It("Then KCP Kyma CR should be requeued and gets into \"Ready\" State", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
			Consistently(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})
	})
})
