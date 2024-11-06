package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ModuleReleaseMeta Sync", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	v1Version := "1.1.1-e2e-test"
	InitEmptyKymaBeforeAll(kyma)

	Context("Given SKR Cluster with  ModuleTemplate", func() {
		It("When Template Operator v1 ModuleTemplate is applied in the KCP Cluster with ModuleReleaseMeta", func() {
			By("Then the Template Operator v1 ModuleTemplate exists in the KCP Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(kcpClient, module, v1beta2.DefaultChannel).
				Should(Succeed())

			By("And the Template Operator v1 ModuleTemplate exists in the SKR Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, v1beta2.DefaultChannel).
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

		It("When Template Operator v1 ModuleTemplate is removed from the KCP Cluster", func() {
			Eventually(DeleteModuleTemplate).
				WithContext(ctx).
				WithArguments(kcpClient, module, v1beta2.DefaultChannel).
				Should(Succeed())

			By("Then Template Operator v1 ModuleTemplate no longer exists on the KCP Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(kcpClient, module, v1beta2.DefaultChannel).
				Should(Equal(ErrNotFound))

			By("Then Template Operator v1 ModuleTemplate no longer exists on the SKR Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, v1beta2.DefaultChannel).
				Should(Equal(ErrNotFound))
		})

		It("When Template Operator v2 ModuleTemplate is applied in the KCP Cluster", func() {
			v2Version := "2.4.2-e2e-test"
			By("And ModuleReleaseMeta is updated with the correct channel-version")
			Eventually(UpdateChannelVersionIfModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(kcpClient, module.Name, ControlPlaneNamespace, v1beta2.DefaultChannel, v2Version).
				Should(Succeed())

			By("Then the Template Operator v2 ModuleTemplate exists in the KCP Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(kcpClient, module, v1beta2.DefaultChannel).
				Should(Succeed())

			By("And the Template Operator v2 ModuleTemplate exists in the SKR Cluster")
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, v1beta2.DefaultChannel).
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
		})
	})
})
