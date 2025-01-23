package e2e_test

import (
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ModuleReleaseMeta Sync", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	v1Version := "1.1.1-e2e-test"
	v2Version := "2.4.2-e2e-test"
	InitEmptyKymaBeforeAll(kyma)

	Context("Given SKR Cluster with  ModuleTemplate", func() {
		It("When Template Operator v1 ModuleTemplate is applied in the KCP Cluster with ModuleReleaseMeta", func() {
			By("Then the Template Operator v1 ModuleTemplate exists in the KCP Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(kcpClient, module, v1beta2.DefaultChannel, ControlPlaneNamespace).
				Should(Succeed())

			Eventually(ImmediatelyRequeueKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace).
				Should(Succeed())

			By("And the Template Operator v1 ModuleTemplate exists in the SKR Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, v1beta2.DefaultChannel, RemoteNamespace).
				Should(Succeed())

			By("And the ModuleReleaseMeta exists on the KCP Cluster with the correct channel-version")
			Eventually(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, ControlPlaneNamespace, kcpClient).
				Should(Succeed())

			Eventually(ModuleReleaseMetaContainsCorrectChannelVersion).
				WithContext(ctx).
				WithArguments(module.Name, ControlPlaneNamespace, v1beta2.DefaultChannel, v1Version, kcpClient).
				Should(Succeed())

			By("And the ModuleReleaseMeta exists on the SKR Cluster with the correct channel-version")
			Eventually(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, skrClient).
				Should(Succeed())

			Eventually(ModuleReleaseMetaContainsCorrectChannelVersion).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, v1beta2.DefaultChannel, v1Version, skrClient).
				Should(Succeed())

			By("And the ModuleReleaseMeta has the correct beta and internal values on the SKR Cluster")
			Eventually(ModuleReleaseMetaBetaValueIsCorrect).
				WithContext(ctx).
				WithArguments(skrClient, RemoteNamespace, module.Name, false).
				Should(Succeed())

			Eventually(ModuleReleaseMetaInternalValueIsCorrect).
				WithContext(ctx).
				WithArguments(skrClient, RemoteNamespace, module.Name, false).
				Should(Succeed())
		})

		It("When Beta value is set to true on the SKR Cluster", func() {
			Eventually(SetModuleReleaseMetaBeta).
				WithContext(ctx).
				WithArguments(true, module.Name, RemoteNamespace, skrClient).
				Should(Succeed())
		})

		It("Then Beta value is reverted back to its value from the KCP Cluster", func() {
			Eventually(ModuleReleaseMetaBetaValueIsCorrect).
				WithContext(ctx).
				WithArguments(skrClient, RemoteNamespace, module.Name, false).
				Should(Succeed())
		})

		It("When Internal value is set to true on the SKR Cluster", func() {
			Eventually(SetModuleReleaseMetaInternal).
				WithContext(ctx).
				WithArguments(true, module.Name, RemoteNamespace, skrClient).
				Should(Succeed())
		})

		It("Then Internal value is reverted back to its value from the KCP Cluster", func() {
			Eventually(ModuleReleaseMetaInternalValueIsCorrect).
				WithContext(ctx).
				WithArguments(skrClient, RemoteNamespace, module.Name, false).
				Should(Succeed())
		})

		It("When the ModuleReleaseMeta is set to beta", func() {
			Eventually(SetModuleReleaseMetaBeta).
				WithContext(ctx).
				WithArguments(true, module.Name, ControlPlaneNamespace, kcpClient).
				Should(Succeed())
			Eventually(ImmediatelyRequeueKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace).
				Should(Succeed())

			By("Then the ModuleReleaseMeta no longer exists on the SKR Cluster")
			Eventually(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, skrClient).
				Should(Equal(ErrNotFound))

			By("And the Template Operator v1 ModuleTemplate no longer exists in the SKR Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, v1beta2.DefaultChannel, RemoteNamespace).
				Should(Equal(ErrNotFound))
		})

		It("When the Kyma is set to beta", func() {
			Eventually(UpdateKymaLabel).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace, shared.BetaLabel, shared.EnableLabelValue).
				Should(Succeed())

			By("Then the ModuleReleaseMeta exists on the SKR Cluster")
			Eventually(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, skrClient).
				Should(Succeed())

			By("And the Template Operator v1 ModuleTemplate exists in the SKR Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, v1beta2.DefaultChannel, RemoteNamespace).
				Should(Succeed())
		})

		It("When the ModuleReleaseMeta is set to internal", func() {
			Eventually(SetModuleReleaseMetaInternal).
				WithContext(ctx).
				WithArguments(true, module.Name, ControlPlaneNamespace, kcpClient).
				Should(Succeed())
			Eventually(ImmediatelyRequeueKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace).
				Should(Succeed())

			By("Then the ModuleReleaseMeta no longer exists on the SKR Cluster")
			Eventually(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, skrClient).
				Should(Equal(ErrNotFound))

			By("And the Template Operator v1 ModuleTemplate no longer exists in the SKR Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, v1beta2.DefaultChannel, RemoteNamespace).
				Should(Equal(ErrNotFound))
		})

		It("When the Kyma is set to internal", func() {
			Eventually(UpdateKymaLabel).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace, shared.InternalLabel, shared.EnableLabelValue).
				Should(Succeed())

			By("Then the ModuleReleaseMeta exists on the SKR Cluster")
			Eventually(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, skrClient).
				Should(Succeed())

			By("And the Template Operator v1 ModuleTemplate exists in the SKR Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, v1beta2.DefaultChannel, RemoteNamespace).
				Should(Succeed())
		})

		It("When Template Operator v1 ModuleTemplate is removed from the KCP Cluster", func() {
			Eventually(DeleteModuleTemplate).
				WithContext(ctx).
				WithArguments(kcpClient, module, v1beta2.DefaultChannel, ControlPlaneNamespace).
				Should(Succeed())

			By("Then Template Operator v1 ModuleTemplate no longer exists on the KCP Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(kcpClient, module, v1beta2.DefaultChannel, ControlPlaneNamespace).
				Should(Equal(ErrNotFound))

			By("Then Template Operator v1 ModuleTemplate no longer exists on the SKR Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, v1beta2.DefaultChannel, RemoteNamespace).
				Should(Equal(ErrNotFound))
		})

		It("When Template Operator v2 ModuleTemplate is applied in the KCP Cluster", func() {
			By("And ModuleReleaseMeta is updated with the correct channel-version")
			Eventually(UpdateChannelVersionInModuleReleaseMeta).
				WithContext(ctx).
				WithArguments(kcpClient, module.Name, ControlPlaneNamespace, v1beta2.DefaultChannel, NewerVersion).
				Should(Succeed())
			Eventually(ImmediatelyRequeueKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace).
				Should(Succeed())

			By("Then the Template Operator v2 ModuleTemplate exists in the KCP Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(kcpClient, module, v1beta2.DefaultChannel, ControlPlaneNamespace).
				Should(Succeed())

			By("And the Template Operator v2 ModuleTemplate exists in the SKR Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, v1beta2.DefaultChannel, RemoteNamespace).
				Should(Succeed())

			By("And the ModuleReleaseMeta exists on the KCP Cluster with the correct channel-version")
			Eventually(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, ControlPlaneNamespace, kcpClient).
				Should(Succeed())

			Eventually(ModuleReleaseMetaContainsCorrectChannelVersion).
				WithContext(ctx).
				WithArguments(module.Name, ControlPlaneNamespace, v1beta2.DefaultChannel, NewerVersion, kcpClient).
				Should(Succeed())

			By("And the ModuleReleaseMeta exists on the SKR Cluster with the correct channel-version")
			Eventually(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, skrClient).
				Should(Succeed())

			Eventually(ModuleReleaseMetaContainsCorrectChannelVersion).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, v1beta2.DefaultChannel, NewerVersion, skrClient).
				Should(Succeed())
		})

		It("When the ModuleReleaseMeta is deleted for the Template Operator Module", func() {
			Eventually(DeleteModuleReleaseMeta).
				WithContext(ctx).
				WithArguments(module.Name, ControlPlaneNamespace, kcpClient).
				Should(Succeed())
			Eventually(ImmediatelyRequeueKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace).
				Should(Succeed())

			By("Then the ModuleReleaseMeta no longer exists on the KCP Cluster")
			Eventually(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, ControlPlaneNamespace, kcpClient).
				Should(Equal(ErrNotFound))

			By("And the ModuleReleaseMeta no longer exists on the SKR Cluster")
			Eventually(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, skrClient).
				Should(Equal(ErrNotFound))

			By("Then the Template Operator v2 ModuleTemplate exists in the KCP Cluster")
			Eventually(ModuleTemplateExistsByName).
				WithContext(ctx).
				WithArguments(kcpClient, fmt.Sprintf("%s-%s", module.Name, v2Version), ControlPlaneNamespace).
				Should(Succeed())

			By("And Template Operator v2 ModuleTemplate no longer exists on the SKR Cluster")
			Eventually(ModuleTemplateExistsByName).
				WithContext(ctx).
				WithArguments(skrClient, fmt.Sprintf("%s-%s", module.Name, v2Version), RemoteNamespace).
				Should(Equal(ErrNotFound))
		})
	})
})
