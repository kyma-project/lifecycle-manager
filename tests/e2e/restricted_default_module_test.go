package e2e_test

import (
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
)

var _ = Describe("Restricted Default Modules", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	// this global account id is added to the module release meta in the Makefile
	kyma.Labels["kyma-project.io/global-account-id"] = GlobalAccountID2
	moduleCR := NewTestModuleCR(RemoteNamespace)

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given SKR Cluster", func() {
		// InitEmptyKymaBeforeAll verified that the Kyma CR is in Ready state on both SKR and KCP
		It("Then the matching restricted module is enabled on the KCP cluster", func() {
			Eventually(ContainsModuleInSpec).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), DeployerModuleName).
				Should(Succeed())
			By("And the non-matching restricted module is not enabled on the KCP cluster")
			Eventually(NotContainsModuleInSpec).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), "not-"+DeployerModuleName).
				Should(Succeed())
			By("And the Module CR has been installed on the SKR cluster")
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Succeed())
			By("And the Module Operator Deployment is ready on the SKR cluster")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInOlderVersion, TestModuleResourceNamespace).
				Should(Succeed())
		})

		It("When the restricted module is disabled on the SKR cluster", func() {
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, DeployerModuleName).
				Should(Succeed())

			By("Then the Module CR is removed from the SKR cluster")
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Equal(ErrNotFound))

			By("And the Module Operator Deployment is removed from the SKR cluster")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInOlderVersion, TestModuleResourceNamespace).
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

		It("When the restricted module is enabled again on the SKR cluster", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace,
					NewDeployer(v1beta2.DefaultChannel)).
				Should(Succeed())

			By("And the Module CR has been installed on the SKR cluster")
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

		It("When the Global Account is removed from the MRM kymaSelector", func() {
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

		It("Then the module is forcefully uninstalled from the SKR cluster", func() {
			By("And the Module CR is removed from the SKR cluster")
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Equal(ErrNotFound))

			By("And the Module Operator Deployment is removed from the SKR cluster")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInOlderVersion, TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))

			By("And KCP Kyma CR is in \"Error\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateError).
				Should(Succeed())

			By("And the SKR Kyma status contains the expected error message for the module")
			Eventually(ModuleMessageInKymaStatusIsCorrect).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, TestModuleName,
					"module template not allowed: restricted module not allowed for this Kyma").
				Should(Succeed())
		})
	})
})
