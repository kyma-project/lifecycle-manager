package e2e_test

import (
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("1 Deployer Module Image Pull Secret Injection", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	// this global account id is added to the module release meta in the Makefile
	kyma.Labels["kyma-project.io/global-account-id"] = GlobalAccountID2
	moduleCR := NewTestModuleCR(RemoteNamespace)

	imagePullResourceName := "image-pull-secret"
	injectFromKCPTransform := "inject-data-from-kcp transform failed"
	errMsgModuleLabelMismatch := fmt.Sprintf(
		"%s: kcp source secret %s label does not match the manifest module"+
			": secret=%s expected=deployer actual=not-deployer",
		injectFromKCPTransform, shared.ModuleName, imagePullResourceName)
	errMsgMissingModuleLabel := fmt.Sprintf(
		"%s: kcp source secret is missing the required %s label: secret=%s",
		injectFromKCPTransform, shared.ModuleName, imagePullResourceName)
	errMsgFetchKCPSecret := fmt.Sprintf(
		"%s: failed to fetch kcp secret %q: failed to get secret %s-kcp-system: secrets %q not found",
		injectFromKCPTransform, imagePullResourceName, imagePullResourceName, imagePullResourceName)

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
					imagePullResourceName,
					TestModuleResourceNamespace,
					".dockerconfigjson",
					[]byte(`{"auths":{"e2e-test.example.com":{"username":"test","password":"test"}}}`+"\n"),
				).
				WithTimeout(ConsistentDuration).
				WithPolling(interval).
				Should(Succeed())
		})

		It("Then a change to the KCP secret data eventually propagates to the SKR", func() {
			updatedDockerConfig := []byte(
				`{"auths":{"e2e-test.example.com":{"username":"updated","password":"updated"}}}` + "\n")
			By("When the KCP image-pull-secret data is updated")
			Expect(UpdateSecretDataKey(ctx, kcpClient, imagePullResourceName,
				shared.DefaultControlPlaneNamespace, ".dockerconfigjson", updatedDockerConfig)).To(Succeed())
			By("Then the updated data eventually arrives at the SKR image-pull-secret")
			Eventually(SecretDataEquals).
				WithContext(ctx).
				WithArguments(
					skrClient,
					imagePullResourceName,
					TestModuleResourceNamespace,
					".dockerconfigjson",
					updatedDockerConfig,
				).
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
				Expect(UpdateSecretLabel(ctx, kcpClient, imagePullResourceName,
					shared.DefaultControlPlaneNamespace, shared.ModuleName, "not-deployer")).To(Succeed())
				By("Then the Manifest reports a module label mismatch error")
				Eventually(ManifestStatusOperationContainsMessage).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), DeployerModuleName,
						errMsgModuleLabelMismatch).
					Should(Succeed())
				Consistently(ManifestStatusOperationContainsMessage).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), DeployerModuleName,
						errMsgModuleLabelMismatch).
					WithTimeout(ConsistentDuration).
					WithPolling(interval).
					Should(Succeed())
			})

		It("Then with the KCP secret module-name label removed, the Manifest reports a missing module label error",
			func() {
				By("When the KCP image-pull-secret module-name label is removed entirely")
				Expect(RemoveSecretLabel(ctx, kcpClient, imagePullResourceName,
					shared.DefaultControlPlaneNamespace, shared.ModuleName)).To(Succeed())
				By("Then the Manifest reports a missing module label error")
				Eventually(ManifestStatusOperationContainsMessage).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), DeployerModuleName,
						errMsgMissingModuleLabel).
					Should(Succeed())
				Consistently(ManifestStatusOperationContainsMessage).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), DeployerModuleName,
						errMsgMissingModuleLabel).
					WithTimeout(ConsistentDuration).
					WithPolling(interval).
					Should(Succeed())
			})
		It("Then with the KCP secret deleted, the Manifest reports a failure to fetch the KCP secret",
			func() {
				By("When the KCP image-pull-secret is deleted")
				Expect(DeleteAnySecret(ctx, imagePullResourceName, kcpClient)).To(Succeed())
				By("Then the Manifest reports a failure to fetch the KCP secret")
				Eventually(ManifestStatusOperationContainsMessage).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), DeployerModuleName,
						errMsgFetchKCPSecret).
					Should(Succeed())
				Consistently(ManifestStatusOperationContainsMessage).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), DeployerModuleName,
						errMsgFetchKCPSecret).
					WithTimeout(ConsistentDuration).
					WithPolling(interval).
					Should(Succeed())
			})
	})
})
