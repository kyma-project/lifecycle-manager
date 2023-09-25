//go:build deletion_e2e

package e2e_test

import (
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
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

var _ = Describe("KCP Kyma CR should be deleted successfully when SKR cluster gets deleted",
	Ordered, func() {
		channel := "regular"
		kyma := testutils.NewKymaForE2E("kyma-sample", "kcp-system", channel)
		remoteNamespace := "kyma-system"
		GinkgoWriter.Printf("kyma before create %v\n", kyma)

		BeforeAll(func() {
			// make sure we can list Kymas to ensure CRDs have been installed
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

			cmd = exec.Command("k3d", "kubeconfig", "write", "skr")
			out, err = cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf(string(out))

			controlPlaneConfig, runtimeConfig, err = getKubeConfigs()
			Expect(err).ToNot(HaveOccurred())

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

		It("Should delete KCP Kyma", func() {
			By("Deleting KCP Kyma")
			Eventually(controlPlaneClient.Delete, readyTimeout, interval).
				WithContext(ctx).
				WithArguments(kyma).
				Should(Succeed())
		})

		It("Kyma CR should be removed", func() {
			Eventually(CheckKCPKymaCRDeleted, timeout, interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())
		})
	})

var _ = Describe("When KCP Kyma CR deleted, Kyma Status Metric should also be deleted",
	Ordered, func() {
		channel := "regular"
		kyma := testutils.NewKymaForE2E("kyma-sample", "kcp-system", channel)
		remoteNamespace := "kyma-system"
		GinkgoWriter.Printf("kyma before create %v\n", kyma)

		BeforeAll(func() {
			// make sure we can list Kymas to ensure CRDs have been installed
			err := controlPlaneClient.List(ctx, &v1beta2.KymaList{})
			Expect(meta.IsNoMatchError(err)).To(BeFalse())
		})

		It("Should create empty Kyma CR", func() {
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

		It("Kyma reconciliation should remove metric when Kyma CR deleted ", func() {
			By("getting the current metric count")
			metricCountBeforeDeletion, err := getKymaStateMetricCount()
			Expect(err).Should(Not(HaveOccurred()))
			GinkgoWriter.Printf("Kyma State Metric count before CR deletion: %d", metricCountBeforeDeletion)

			By("deleting KCP Kyma")
			Eventually(controlPlaneClient.Delete, readyTimeout, interval).
				WithContext(ctx).
				WithArguments(kyma).
				Should(Succeed())

			By("waiting for Kyma CR to be removed")
			Eventually(CheckKCPKymaCRDeleted, 2*time.Minute, interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())

			By("should decrease the metric count")
			metricCountAfterDeletion, err := getKymaStateMetricCount()
			Expect(err).Should(Not(HaveOccurred()))
			GinkgoWriter.Printf("Kyma State Metric count after CR deletion: %d", metricCountAfterDeletion)
			Expect(metricCountAfterDeletion).To(Equal(metricCountBeforeDeletion - 1))
		})
	})

func getKymaStateMetricCount() (int, error) {
	response, err := http.Get("http://localhost:9081/metrics")
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return 0, err
	}
	bodyString := string(bodyBytes)

	sum := 0
	states := []string{"Deleting", "Error", "Processing", "Ready", "Warning"}
	for _, state := range states {
		re := regexp.MustCompile(
			`lifecycle_mgr_kyma_state{instance_id="[^"]+",kyma_name="[^"]+",shoot="[^"]+",state="` +
				state + `"} (\d+)`)
		match := re.FindStringSubmatch(bodyString)
		if len(match) > 1 {
			count, err := strconv.Atoi(match[1])
			if err != nil {
				return 0, err
			}
			sum += count
		}
	}
	return sum, nil
}
