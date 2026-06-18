package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
)

const notDeployerModuleName = "not-deployer"

var _ = Describe("Block Non Deployer Module Image Pull Secret Injection", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	// this global account id is added to the module release meta in the Makefile
	kyma.Labels["kyma-project.io/global-account-id"] = GlobalAccountID2
	moduleCR := NewTestModuleCR(RemoteNamespace)

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given SKR Cluster", func() {
		It("Then the not-deployer module is installed as a restricted default module", func() {
			Eventually(ContainsModuleInSpec).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), notDeployerModuleName).
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

		It("Then the annotated image-pull-secret on the SKR is not patched", func() {
			Consistently(SecretDataEquals).
				WithContext(ctx).
				WithArguments(
					skrClient,
					"image-pull-secret",
					TestModuleResourceNamespace,
					".dockerconfigjson",
					[]byte(`{"auths": {}}`+"\n"),
				).
				WithTimeout(ConsistentDuration).
				WithPolling(interval).
				Should(Succeed())
		})
	})
})
