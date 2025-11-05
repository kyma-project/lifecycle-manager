package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = FDescribe("SKR Image Pull Secret Sync", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)

	const skrImagePullSecretName = "some-special-image-pull-secret"

	BeforeAll(func() {
		By("When some Secret is created as SKR Image Pull Secret in the KCP Cluster")
		Eventually(CreateAnySecret).
			WithContext(ctx).
			WithArguments(skrImagePullSecretName, kcpClient).
			Should(Succeed())
	})

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given SKR Cluster", func() {
		It("When SKR image pull Secret exists in the KCP Cluster", func() {
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments(skrImagePullSecretName, ControlPlaneNamespace, "", "v1", "Secret", kcpClient).
				Should(Succeed())
		})

		It("And skr-image-pull-secret flag is set in Deployment", func() {
			Eventually(DeploymentContainerHasFlag).
				WithContext(ctx).
				WithArguments("klm-controller-manager", "kcp-system", "skr-image-pull-secret",
					skrImagePullSecretName, kcpClient).
				Should(Succeed())
		})

		It("Then SKR cluster should have that Secret synced", func() {
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments(skrImagePullSecretName, RemoteNamespace, "", "v1", "Secret", skrClient).
				Should(Succeed())
		})

		It("When Kyma Module is enabled on SKR Kyma CR", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())
		})

		It("Then imagePullSecrets in Deployment should contain the SKR image pull Secret", func() {
			Eventually(DeploymentPodSpecHasImagePullSecret).
				WithContext(ctx).
				WithArguments("template-operator-v1-controller-manager", "template-operator-system",
					skrImagePullSecretName, skrClient).
				Should(Succeed())
		})

		It("And env in Deployment containers have the SKR_IMG_PULL_SECRET var", func() {
			Eventually(DeploymentContainersHaveImagePullSecretEnv).
				WithContext(ctx).
				WithArguments("template-operator-v1-controller-manager", "template-operator-system",
					skrImagePullSecretName, skrClient).
				Should(Succeed())
		})
	})
})
