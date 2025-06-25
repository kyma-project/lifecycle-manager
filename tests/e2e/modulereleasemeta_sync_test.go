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
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	var skrKyma *v1beta2.Kyma
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	v1Version := "1.1.1-e2e-test"
	v2Version := "2.4.2-e2e-test"
	InitEmptyKymaBeforeAll(kyma)

	Context("Given SKR Cluster", func() {
		It("When Kyma can be fetched from SKR Cluster", func() {
			var err error
			skrKyma, err = GetKyma(ctx, skrClient, shared.DefaultRemoteKymaName, RemoteNamespace)
			if err != nil {
				Fail("Failed to get SKR Kyma")
			}
		})

		It("When Template Operator v1 ModuleTemplate is applied in the KCP Cluster with ModuleReleaseMeta", func() {
			By("Then the Template Operator v1 ModuleTemplate exists in the KCP Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(kcpClient, module, kyma).
				Should(Succeed())

			Eventually(ImmediatelyRequeueKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace).
				Should(Succeed())

			By("And the Template Operator v1 ModuleTemplate exists in the SKR Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, skrKyma).
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
		})

		It("When the Template Operator v1 ModuleTemplate is set to beta", func() {
			Eventually(SetModuleTemplateBetaLabel).
				WithContext(ctx).
				WithArguments(kcpClient, module, kyma, true).
				Should(Succeed())
			Eventually(ImmediatelyRequeueKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace).
				Should(Succeed())

			By("Then the ModuleReleaseMeta still exists in the SKR Cluster but only with fast channel mapped with v2")
			Eventually(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, skrClient).
				Should(Succeed())
			Eventually(ModuleReleaseMetaContainsCorrectChannelVersion).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, "fast", v2Version, skrClient).
				Should(Succeed())
			Eventually(ModuleReleaseMetaContainsCorrectChannelVersion).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, v1beta2.DefaultChannel, v1Version, skrClient).
				Should(Equal(ErrNotExpectedChannelVersion))

			By("And the Template Operator v1 ModuleTemplate no longer exists in the SKR Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, skrKyma).
				Should(Equal(ErrNotFound))
		})

		It("When the Template Operator v2 ModuleTemplate is set to beta", func() {
			module.Channel = "fast"
			Eventually(SetModuleTemplateBetaLabel).
				WithContext(ctx).
				WithArguments(kcpClient, module, kyma, true).
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

			By("And the Template Operator v2 ModuleTemplate no longer exists in the SKR Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, skrKyma).
				Should(Equal(ErrNotFound))
			module.Channel = v1beta2.DefaultChannel
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
				WithArguments(skrClient, module, skrKyma).
				Should(Succeed())
		})

		It("When the Template Operator v1 ModuleTemplate is set to internal", func() {
			Eventually(SetModuleTemplateInternalLabel).
				WithContext(ctx).
				WithArguments(kcpClient, module, kyma, true).
				Should(Succeed())
			Eventually(ImmediatelyRequeueKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace).
				Should(Succeed())

			By("Then the ModuleReleaseMeta still exists in the SKR Cluster but only with fast channel mapped with v2")
			Eventually(ModuleReleaseMetaContainsCorrectChannelVersion).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, "fast", v2Version, skrClient).
				Should(Succeed())
			Eventually(ModuleReleaseMetaContainsCorrectChannelVersion).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, v1beta2.DefaultChannel, v2Version, skrClient).
				Should(Equal(ErrNotExpectedChannelVersion))

			By("And the Template Operator v1 ModuleTemplate no longer exists in the SKR Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, skrKyma).
				Should(Equal(ErrNotFound))
		})

		It("When the Template Operator v2 ModuleTemplate is set to internal", func() {
			module.Channel = "fast"
			Eventually(SetModuleTemplateInternalLabel).
				WithContext(ctx).
				WithArguments(kcpClient, module, kyma, true).
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

			By("And the Template Operator v2 ModuleTemplate no longer exists in the SKR Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, skrKyma).
				Should(Equal(ErrNotFound))
			module.Channel = v1beta2.DefaultChannel
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
				WithArguments(skrClient, module, skrKyma).
				Should(Succeed())
		})

		It("the Template Operator v1 ModuleTemplate beta and internal values are set to false", func() {
			Eventually(SetModuleTemplateInternalLabel).
				WithContext(ctx).
				WithArguments(kcpClient, module, kyma, false).
				Should(Succeed())
			Eventually(SetModuleTemplateBetaLabel).
				WithContext(ctx).
				WithArguments(kcpClient, module, kyma, false).
				Should(Succeed())
			Eventually(ImmediatelyRequeueKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace).
				Should(Succeed())

			By("And Template Operator v1 ModuleTemplate is removed from the KCP Cluster")
			Eventually(DeleteModuleTemplate).
				WithContext(ctx).
				WithArguments(kcpClient, module, kyma).
				Should(Succeed())

			By("Then Template Operator v1 ModuleTemplate no longer exists on the KCP Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(kcpClient, module, kyma).
				Should(Equal(ErrNotFound))

			By("Then Template Operator v1 ModuleTemplate no longer exists on the SKR Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, skrKyma).
				Should(Equal(ErrNotFound))
		})

		It("When ModuleReleaseMeta is updated with the v2 version mapped to regular channel", func() {
			Eventually(UpdateChannelVersionInModuleReleaseMeta).
				WithContext(ctx).
				WithArguments(kcpClient, module.Name, ControlPlaneNamespace, v1beta2.DefaultChannel, v2Version).
				Should(Succeed())
			Eventually(ImmediatelyRequeueKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace).
				Should(Succeed())

			By("And the Template Operator v2 ModuleTemplate beta and internal values are set to false")
			Eventually(SetModuleTemplateInternalLabel).
				WithContext(ctx).
				WithArguments(kcpClient, module, kyma, false).
				Should(Succeed())
			Eventually(SetModuleTemplateBetaLabel).
				WithContext(ctx).
				WithArguments(kcpClient, module, kyma, false).
				Should(Succeed())
			Eventually(ImmediatelyRequeueKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace).
				Should(Succeed())

			By("Then the Template Operator v2 ModuleTemplate exists in the KCP Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(kcpClient, module, kyma).
				Should(Succeed())

			By("And the Template Operator v2 ModuleTemplate exists in the SKR Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, skrKyma).
				Should(Succeed())

			By("And the ModuleReleaseMeta exists on the KCP Cluster with the correct channel-version")
			Eventually(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, ControlPlaneNamespace, kcpClient).
				Should(Succeed())

			Eventually(ModuleReleaseMetaContainsCorrectChannelVersion).
				WithContext(ctx).
				WithArguments(module.Name, ControlPlaneNamespace, v1beta2.DefaultChannel, v2Version, kcpClient).
				Should(Succeed())

			By("And the ModuleReleaseMeta exists on the SKR Cluster with the correct channel-version")
			Eventually(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, skrClient).
				Should(Succeed())

			Eventually(ModuleReleaseMetaContainsCorrectChannelVersion).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, v1beta2.DefaultChannel, v2Version, skrClient).
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
