//go:build deletion_e2e

package e2e_test

import (
	"context"
	"errors"
	"os/exec"
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errKymaNotDeleted = errors.New("kyma CR not deleted")
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
		})

		It("Should delete KCP Kyma", func() {
			By("Kyma should be in Error state")
			Eventually(CheckKymaIsInState, readyTimeout, interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateError).
				Should(Succeed())

			By("Deleting KCP Kyma")
			Eventually(controlPlaneClient.Delete, readyTimeout, interval).
				WithContext(ctx).
				WithArguments(kyma).
				Should(Succeed())
		})

		It("Kyma CR should be removed", func() {
			Eventually(checkKCPKymaCRDeleted, timeout, interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())
		})
	})

func checkKCPKymaCRDeleted(ctx context.Context,
	kymaName string, kymaNamespace string, k8sClient client.Client,
) error {
	kyma := &v1beta2.Kyma{}
	err := k8sClient.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, kyma)
	if util.IsNotFound(err) {
		return nil
	}
	return errKymaNotDeleted
}
