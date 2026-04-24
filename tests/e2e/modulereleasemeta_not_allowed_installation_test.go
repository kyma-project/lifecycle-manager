package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
)

var notAllowedInstallationLabelSelector = map[string]string{"foo": "bar", "baz": "cux"}

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

		It("When the ModuleTemplate internal label is removed to restore module to Ready state", func() {
			Eventually(SetModuleTemplateInternalLabel).
				WithContext(ctx).
				WithArguments(kcpClient, module, kyma, false).
				Should(Succeed())
		})
		It("Then the module is in Ready state again", func() {
			Eventually(CheckModuleState).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name, shared.StateReady).
				Should(Succeed())
			By("And the KCP Kyma CR is in a \"Ready\" State", func() {
				Eventually(KymaIsInState).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
					Should(Succeed())
			})
		})

		It("When a non-matching KymaLabelSelector is added to the ModuleReleaseMeta", func() {
			Eventually(func() error {
				return SetMandatoryModuleReleaseMetaKymaLabelSelector(ctx, kcpClient,
					module.Name, ControlPlaneNamespace,
					&apimetav1.LabelSelector{
						MatchLabels: notAllowedInstallationLabelSelector,
					})
			}).Should(Succeed())
		})
		It("Then the module is uninstalled from the SKR cluster and in error state", func() {
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInNewerVersion,
					TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))
			By("And the module is in error state", func() {
				Eventually(CheckModuleState).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name, shared.StateError).
					Should(Succeed())
			})
		})

		It("When the matching labels are added to the Kyma CR", func() {
			Eventually(func() error {
				for key, value := range notAllowedInstallationLabelSelector {
					if err := UpdateKymaLabel(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace(), key, value); err != nil {
						return err
					}
				}
				return nil
			}).Should(Succeed())
		})
		It("Then the module is installed again on the SKR cluster", func() {
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInNewerVersion,
					TestModuleResourceNamespace).
				Should(Succeed())
			By("And the module is in Ready state", func() {
				Eventually(CheckModuleState).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name, shared.StateReady).
					Should(Succeed())
			})
			By("And the KCP Kyma CR is in a \"Ready\" State", func() {
				Eventually(KymaIsInState).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
					Should(Succeed())
			})
		})

		It("When the KymaLabelSelector is removed from the ModuleReleaseMeta", func() {
			Eventually(func() error {
				return SetMandatoryModuleReleaseMetaKymaLabelSelector(ctx, kcpClient,
					module.Name, ControlPlaneNamespace, nil)
			}).Should(Succeed())
		})
		It("Then the module remains installed on the SKR cluster", func() {
			Consistently(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInNewerVersion,
					TestModuleResourceNamespace).
				Should(Succeed())
		})
	})
})
