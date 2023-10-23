package e2e_test

import (
	"os/exec"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("KCP Kyma CR Deletion", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", "regular",
		v1beta2.SyncStrategyLocalSecret)

	InitEmptyKymaBeforeAll(kyma)

	It("Should remove SKR Cluster", func() {
		By("removing SKR Cluster")
		cmd := exec.Command("k3d", "cluster", "rm", "skr")
		out, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf(string(out))

		By("deleting secret")
		Eventually(DeleteKymaSecret).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
			Should(Succeed())

		By("Kyma should be in Error state")
		Eventually(IsKymaInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateError).
			Should(Succeed())
	})

	It("Should recreate SKR cluster and its secret", func() {
		By("Creating SKR cluster")
		cmd := exec.Command("k3d", "cluster", "create", "skr")
		out, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf(string(out))

		cmd = exec.Command("k3d", "kubeconfig", "write", "skr")
		out, err = cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf(string(out))

		controlPlaneConfig, runtimeConfig, err = getKubeConfigs()
		Expect(err).ToNot(HaveOccurred())

		By("Creating Kyma secret")
		Eventually(CreateKymaSecret).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
			Should(Succeed())
	})

	It("Kyma should be in Ready state after secret is re-fetched", func() {
		By("Kyma should be in Ready state")
		Eventually(IsKymaInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
			Should(Succeed())
	})

	It("Should delete KCP Kyma", func() {
		By("Deleting KCP Kyma")
		Eventually(controlPlaneClient.Delete).
			WithContext(ctx).
			WithArguments(kyma).
			Should(Succeed())
	})

	It("Kyma CR should be removed", func() {
		Eventually(KymaDeleted).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
			Should(Succeed())
	})
})
