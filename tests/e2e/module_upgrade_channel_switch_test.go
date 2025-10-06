package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Module Upgrade By Channel Switch", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(RemoteNamespace)
	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given SKR Cluster", func() {
		It("When Kyma Module is enabled on SKR Kyma CR", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())
		})

		It("Then Module CR exists", func() {
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Succeed())

			By("And Module Operator Deployment exists")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInOlderVersion, TestModuleResourceNamespace).
				Should(Succeed())

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())

			By("And Manifest CR is in \"Ready\" State")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient, shared.StateReady).
				Should(Succeed())
		})

		It("When upgrade version by switch Channel", func() {
			Eventually(UpdateKymaModuleChannel).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, FastChannel).
				Should(Succeed())
		})

		It("Then Module CR exists", func() {
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Succeed())

			By("And new Module Operator Deployment exists")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInNewerVersion, TestModuleResourceNamespace).
				Should(Succeed())

			By("And old Module Operator Deployment does not exist")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInOlderVersion, TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())

			By("And Manifest CR is in \"Ready\" State")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient, shared.StateReady).
				Should(Succeed())
		})

		It("When downgrade version by switch Channel", func() {
			Eventually(UpdateKymaModuleChannel).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, v1beta2.DefaultChannel).
				Should(Succeed())
		})

		It("Then Module stay in newer version", func() {
			expectedErrorMessage := "module template update not allowed: " +
				"ignore channel skew (from fast to regular), " +
				"as a higher version (2.4.2-e2e-test) of the module was previously installed"

			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Succeed())

			By("And new Module Operator Deployment exists")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInNewerVersion, TestModuleResourceNamespace).
				Should(Succeed())

			By("And old Module Operator Deployment does not exist")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInOlderVersion, TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))

			By("And KCP Kyma CR is in \"Warning\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateWarning).
				Should(Succeed())

			By("And the Module Status has correct error message", func() {
				Eventually(ModuleMessageInKymaStatusIsCorrect).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name, expectedErrorMessage).
					Should(Succeed())
			})
		})
	})
})
