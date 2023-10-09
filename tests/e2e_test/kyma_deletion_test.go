package e2e_test

import (
	"os/exec"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
)

var _ = Describe("KCP Kyma CR Deletion", Ordered, func() {
	kyma := testutils.NewKymaWithSyncLabel("kyma-sample", "kcp-system", "regular",
		v1beta2.SyncStrategyLocalSecret)
	GinkgoWriter.Printf("kyma before create %v\n", kyma)

	BeforeAll(func() {
		// make sure we can list Kymas to ensure CRDs have been installed
		err := controlPlaneClient.List(ctx, &v1beta2.KymaList{})
		Expect(meta.IsNoMatchError(err)).To(BeFalse())
	})

	It("Should create empty Kyma CR on remote cluster", func() {
		Eventually(CreateKymaSecret).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
			Should(Succeed())
		Eventually(controlPlaneClient.Create).
			WithContext(ctx).
			WithArguments(kyma).
			Should(Succeed())
		By("verifying kyma is ready")
		Eventually(CheckKymaIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
			Should(Succeed())
		By("verifying remote kyma is ready")
		Eventually(CheckRemoteKymaCR).
			WithContext(ctx).
			WithArguments(remoteNamespace, []v1beta2.Module{}, runtimeClient, v1beta2.StateReady).
			Should(Succeed())
	})

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
		Eventually(CheckKymaIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateError).
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
		Eventually(CheckKymaIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
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
		Eventually(CheckKCPKymaCRDeleted).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
			Should(Succeed())
	})
})
