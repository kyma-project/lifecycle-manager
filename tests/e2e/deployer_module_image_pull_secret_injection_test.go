package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
)

var _ = Describe("Deployer Module Image Pull Secret Injection", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	// this global account id is added to the module release meta in the Makefile
	kyma.Labels["kyma-project.io/global-account-id"] = GlobalAccountID2
	moduleCR := NewTestModuleCR(RemoteNamespace)

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given SKR Cluster", func() {
		It("Then the deployer module is enabled as a restricted default module", func() {
			Eventually(ContainsModuleInSpec).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), DeployerModuleName).
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

		It("Then the image-pull-secret on the SKR has data injected from KCP", func() {
			Consistently(SecretDataEquals).
				WithContext(ctx).
				WithArguments(
					skrClient,
					"image-pull-secret",
					TestModuleResourceNamespace,
					".dockerconfigjson",
					[]byte(`{"auths":{"e2e-test.example.com":{"username":"test","password":"test"}}}`+"\n"),
				).
				WithTimeout(ConsistentDuration).
				WithPolling(interval).
				Should(Succeed())
		})

		It("Then the non-annotated pull secret on the SKR retains its original data", func() {
			Consistently(SecretDataEquals).
				WithContext(ctx).
				WithArguments(
					skrClient,
					"non-inject-pull-secret",
					TestModuleResourceNamespace,
					".dockerconfigjson",
					[]byte(`{"auths": {}}`+"\n"),
				).
				WithTimeout(ConsistentDuration).
				WithPolling(interval).
				Should(Succeed())
		})

		It("Then after the KCP secret module-name label is changed, the Manifest reports a module label mismatch error",
			func() {
				By("When the KCP image-pull-secret module-name label is changed to a non-matching value")
				Expect(UpdateSecretLabel(ctx, kcpClient, "image-pull-secret",
					shared.DefaultControlPlaneNamespace, shared.ModuleName, "not-deployer")).To(Succeed())
				By("Then the Manifest reports a module label mismatch error")
				Eventually(ManifestStatusOperationContainsMessage).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), DeployerModuleName,
						"kcp source secret "+shared.ModuleName+" label does not match the manifest module").
					Should(Succeed())
				Consistently(ManifestStatusOperationContainsMessage).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), DeployerModuleName,
						"kcp source secret "+shared.ModuleName+" label does not match the manifest module").
					WithTimeout(ConsistentDuration).
					WithPolling(interval).
					Should(Succeed())
			})

		It("Then with the KCP secret module-name label removed, the Manifest reports a missing module label error",
			func() {
				By("When the KCP image-pull-secret module-name label is removed entirely")
				Expect(RemoveSecretLabel(ctx, kcpClient, "image-pull-secret",
					shared.DefaultControlPlaneNamespace, shared.ModuleName)).To(Succeed())
				By("Then the Manifest reports a missing module label error")
				Eventually(ManifestStatusOperationContainsMessage).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), DeployerModuleName,
						"kcp source secret is missing the required "+shared.ModuleName+" label").
					Should(Succeed())
				Consistently(ManifestStatusOperationContainsMessage).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), DeployerModuleName,
						"kcp source secret is missing the required "+shared.ModuleName+" label").
					WithTimeout(ConsistentDuration).
					WithPolling(interval).
					Should(Succeed())
			})
		It("Then with the KCP secret deleted, the Manifest reports a failure to fetch the KCP secret",
			func() {
				By("When the KCP image-pull-secret is deleted")
				Expect(DeleteAnySecret(ctx, "image-pull-secret", kcpClient)).To(Succeed())
				By("Then the Manifest reports a failure to fetch the KCP secret")
				Eventually(ManifestStatusOperationContainsMessage).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), DeployerModuleName,
						`failed to fetch kcp secret "image-pull-secret"`).
					Should(Succeed())
				Consistently(ManifestStatusOperationContainsMessage).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), DeployerModuleName,
						`failed to fetch kcp secret "image-pull-secret"`).
					WithTimeout(ConsistentDuration).
					WithPolling(interval).
					Should(Succeed())
			})
	})
})
