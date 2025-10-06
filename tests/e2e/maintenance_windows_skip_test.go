package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

/*
Maintenance Windows are defined as such:
	region asia: current time - current time + 2 hours
	region europe: tomorrow
*/

var _ = Describe("Maintenance Windows - No Wait for Maintenance Widnow on Skip", Ordered, func() {
	const europe = "europe"

	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	kyma.Labels[shared.RegionLabel] = europe
	kyma.Spec.SkipMaintenanceWindows = true

	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(RemoteNamespace)

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given SKR Cluster; Kyma CR .spec.skipMaintenanceWindows=true; NO active maintenance window", func() {
		It("When module in regular channel is enabled (requiresDowntime=false)", func() {
			module.Channel = v1beta2.DefaultChannel
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())

			By("Then Module CR exists")
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Succeed())

			By("And correct Module Operator Deployment is deployed")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInOlderVersion, TestModuleResourceNamespace).
				Should(Succeed())

			By("And SKR Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(shared.DefaultRemoteKymaName,
					shared.DefaultRemoteNamespace, skrClient, shared.StateReady).
				Should(Succeed())

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})

		It("When module channel is changed to fast (requiresDowntime=true)", func() {
			module.Channel = FastChannel
			Eventually(UpdateKymaModuleChannel).
				WithContext(ctx).
				WithArguments(skrClient, shared.DefaultRemoteKymaName, shared.DefaultRemoteNamespace, module.Channel).
				Should(Succeed())

			By("Then Module CR exists")
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Succeed())

			By("And Module Operator Deployment from regular channel is NOT deployed")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInOlderVersion, TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))

			By("And Module Operator Deployment from fast channel is deployed")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInNewerVersion, TestModuleResourceNamespace).
				Should(Succeed())

			By("And SKR Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(shared.DefaultRemoteKymaName,
					shared.DefaultRemoteNamespace, skrClient, shared.StateReady).
				Should(Succeed())

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})

		It("When module is disabled again", func() {
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(skrClient, shared.DefaultRemoteKymaName, shared.DefaultRemoteNamespace, module.Name).
				Should(Succeed())

			By("Then Module Operator Deployment from fast channel is removed")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInNewerVersion, TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))

			By("And SKR Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(shared.DefaultRemoteKymaName,
					shared.DefaultRemoteNamespace, skrClient, shared.StateReady).
				Should(Succeed())

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})
	})
})
