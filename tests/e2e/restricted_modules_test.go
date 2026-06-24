package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
)

var _ = Describe("Restricted Modules", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	// The Kyma's global-account-id label matches the deployer MRM's kymaSelector configured by the
	// test's .mk; a later It mutates the kymaSelector so the Kyma no longer matches.
	kyma.Labels["kyma-project.io/global-account-id"] = GlobalAccountID2
	moduleCR := NewTestModuleCR(RemoteNamespace)

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given the Kyma matches the deployer MRM kymaSelector", func() {
		It("When a normal module (template-operator) is enabled", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace,
					NewTemplateOperator(v1beta2.DefaultChannel)).
				Should(Succeed())

			By("Then the Module CR has been installed on the SKR cluster")
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Succeed())
			By("And the Module Operator Deployment is ready on the SKR cluster")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInOlderVersion, TestModuleResourceNamespace).
				Should(Succeed())

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, RemoteNamespace, skrClient, shared.StateReady).
				Should(Succeed())
		})

		It("When the normal module is disabled before the restricted-module lifecycle", func() {
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, TestModuleName).
				Should(Succeed())

			By("Then the Module Operator Deployment is removed from the SKR cluster")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInOlderVersion, TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))

			By("And the Module CR is removed from the SKR cluster")
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Equal(ErrNotFound))

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})

		It("When a non-restricted module's MRM gets a kymaSelector and the module is re-enabled", func() {
			By("Patch the template-operator MRM with a non-matching kymaSelector")
			Eventually(UpdateModuleReleaseMetaKymaSelector).
				WithContext(ctx).
				WithArguments(kcpClient, TestModuleName, ControlPlaneNamespace, &apimetav1.LabelSelector{
					MatchExpressions: []apimetav1.LabelSelectorRequirement{
						{
							Key:      "kyma-project.io/global-account-id",
							Operator: apimetav1.LabelSelectorOpIn,
							Values:   []string{"any-global-account-id"},
						},
					},
				}).
				Should(Succeed())

			By("Re-enable the module on the SKR cluster")
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace,
					NewTemplateOperator(v1beta2.DefaultChannel)).
				Should(Succeed())

			By("Then the module is in error state on the KCP cluster")
			Eventually(CheckModuleState).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, ControlPlaneNamespace, TestModuleName, shared.StateError).
				Should(Succeed())

			By("And SKR Kyma CR is in \"Error\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, RemoteNamespace, skrClient, shared.StateError).
				Should(Succeed())

			By("And the SKR Kyma status contains the expected error message for the module")
			Eventually(ModuleMessageInKymaStatusIsCorrect).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, TestModuleName,
					"module template not allowed: module has kymaSelector but is not in restricted modules list").
				Should(Succeed())
		})

		It("When the rejected normal module is disabled again to clear the error", func() {
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, TestModuleName).
				Should(Succeed())

			By("Then KCP Kyma CR returns to \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, RemoteNamespace, skrClient, shared.StateReady).
				Should(Succeed())
		})

		It("When the deployer module (restricted) is enabled", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace,
					NewDeployer(v1beta2.DefaultChannel)).
				Should(Succeed())

			By("Then the Module CR has been installed on the SKR cluster")
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Succeed())
			By("And the deployer Deployment is ready on the SKR cluster")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, DeployerDeploymentName, TestModuleResourceNamespace).
				Should(Succeed())

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, RemoteNamespace, skrClient, shared.StateReady).
				Should(Succeed())
		})

		It("When the deployer module is disabled", func() {
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, DeployerModuleName).
				Should(Succeed())

			By("Then the Module CR is removed from the SKR cluster")
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Equal(ErrNotFound))

			By("And the deployer Deployment is removed from the SKR cluster")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, DeployerDeploymentName, TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, RemoteNamespace, skrClient, shared.StateReady).
				Should(Succeed())
		})

		It("When the deployer module is re-enabled", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace,
					NewDeployer(v1beta2.DefaultChannel)).
				Should(Succeed())

			By("Then the Module CR has been installed on the SKR cluster")
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Succeed())
			By("And the deployer Deployment is ready on the SKR cluster")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, DeployerDeploymentName, TestModuleResourceNamespace).
				Should(Succeed())

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, RemoteNamespace, skrClient, shared.StateReady).
				Should(Succeed())
		})

		It("When the Global Account is removed from the deployer MRM kymaSelector", func() {
			Eventually(UpdateModuleReleaseMetaKymaSelector).
				WithContext(ctx).
				WithArguments(kcpClient, DeployerModuleName, ControlPlaneNamespace,
					&apimetav1.LabelSelector{
						MatchExpressions: []apimetav1.LabelSelectorRequirement{
							{
								Key:      "kyma-project.io/global-account-id",
								Operator: apimetav1.LabelSelectorOpIn,
								Values:   []string{GlobalAccountID1},
							},
						},
					}).
				Should(Succeed())

			Eventually(ImmediatelyRequeueKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace()).
				Should(Succeed())
		})

		It("Then the deployer module is forcefully uninstalled from the SKR cluster", func() {
			By("And the Module CR is removed from the SKR cluster")
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Equal(ErrNotFound))

			By("And the deployer Deployment is removed from the SKR cluster")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, DeployerDeploymentName, TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))

			By("And KCP Kyma CR is in \"Error\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateError).
				Should(Succeed())

			By("And the SKR Kyma status contains the expected error message for the module")
			Eventually(ModuleMessageInKymaStatusIsCorrect).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, DeployerModuleName,
					"module template not allowed: restricted module not allowed for this Kyma").
				Should(Succeed())
		})
	})
})
