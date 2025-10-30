package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("SKR Image Pull Secret Sync", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)

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
	})
})
