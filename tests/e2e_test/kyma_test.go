//go:build kyma_e2e

package e2e_test

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
)

const (
	timeout       = 10 * time.Second
	statusTimeout = 2 * time.Minute
	interval      = 1 * time.Second
	readyTimeout  = 2 * time.Minute
)

var _ = Describe("SKR Kyma secret should be re-fetched when the SKR cluster is unreachable",
	Ordered, func() {
		channel := "regular"
		kyma := testutils.NewKymaForE2E("kyma-sample", "kcp-system", channel)
		remoteNamespace := "kyma-system"
		GinkgoWriter.Printf("kyma before create %v\n", kyma)

		BeforeAll(func() {
			//make sure we can list Kymas to ensure CRDs have been installed
			err := controlPlaneClient.List(ctx, &v1beta2.KymaList{})
			Expect(meta.IsNoMatchError(err)).To(BeFalse())
		})

		It("Should create empty Kyma CR on remote cluster", func() {
			Eventually(CreateKymaSecret, timeout, interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())
			Eventually(controlPlaneClient.Create, timeout, interval).
				WithContext(ctx).
				WithArguments(kyma).
				Should(Succeed())
			By("verifying kyma is ready")
			Eventually(CheckKymaIsInState, readyTimeout, interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
				Should(Succeed())
			By("verifying remote kyma is ready")
			Eventually(CheckRemoteKymaCR, readyTimeout, interval).
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
			Eventually(DeleteKymaSecret, timeout, interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())

			By("Kyma should be in Error state")
			Eventually(CheckKymaIsInState, readyTimeout, interval).
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

			By("Updating kubeconfig")
			cmd = exec.Command("k3d", "kubeconfig", "merge", "-a", "-d")
			out, err = cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf(string(out))

			//cmd = exec.Command("echo", "\"SKR_KUBECONFIG=$(k3d kubeconfig write skr)\"", ">>", "$GITHUB_ENV")
			cmd = exec.Command("k3d", "kubeconfig", "write", "skr")
			out, err = cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf(string(out))

			cmd = exec.Command("echo", fmt.Sprintf("%s=%s", "SKR_KUBECONFIG", string(out)), ">>",
				"$GITHUB_ENV")
			out, err = cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf(string(out))

			By("Creating Kyma secret")
			Eventually(CreateKymaSecret, timeout, interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())
		})

		It("Kyma should be in Ready state after secret is re-fetched", func() {
			By("Kyma should be in Ready state")
			Eventually(CheckKymaIsInState, readyTimeout, interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
				Should(Succeed())
		})
	})
