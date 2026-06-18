package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
)

var _ = Describe("Deployer Module Image Pull Secret", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	// this global account id is added to the module release meta in the Makefile
	kyma.Labels["kyma-project.io/global-account-id"] = "a1c1d2e3-4a5b-6c7d-8e9f-0a1b2c3d4e5f"
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
			Eventually(SecretDataEquals).
				WithContext(ctx).
				WithArguments(
					skrClient,
					"image-pull-secret",
					TestModuleResourceNamespace,
					".dockerconfigjson",
					[]byte(`{"auths":{"e2e-test.example.com":{"username":"test","password":"test"}}}`+"\n"),
				).
				Should(Succeed())
		})

		It("Then the non-annotated pull secret on the SKR retains its original data", func() {
			Eventually(SecretDataEquals).
				WithContext(ctx).
				WithArguments(
					skrClient,
					"non-inject-pull-secret",
					TestModuleResourceNamespace,
					".dockerconfigjson",
					[]byte(`{"auths": {}}`+"\n"),
				).
				Should(Succeed())
		})
	})
})
