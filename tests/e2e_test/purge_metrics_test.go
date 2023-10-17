package e2e_test

import (
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Purge Metrics", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", "regular",
		v1beta2.SyncStrategyLocalSecret)

	InitEmptyKymaBeforeAll(kyma)

	Context("Given Kyma CR with purge finalizer", func() {
		It("When Kyma deletion timestamp is set", func() {
			Expect(DeleteKyma(ctx, controlPlaneClient, kyma)).
				Should(Succeed())

			Expect(KymaHasDeletionTimestamp(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace())).
				Should(BeTrue())
		})

		It("Then KCP and SKR kymas are deleted after purge timeout", func() {
			time.Sleep(5 * time.Second)
			Eventually(KymaDeleted).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, remoteNamespace, runtimeClient).
				Should(Succeed())

			Eventually(KymaDeleted).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())
		})

		It("And lifecycle_mgr_purgectrl_time is updated", func() {
			duration, err := GetPurgeTimeMetric(ctx)
			Expect(err).NotTo(HaveOccurred())

			GinkgoWriter.Printf("Purge Metric Time: %d", duration)
			Expect(duration).Should(BeNumerically(">", 0))
		})
	})
})
