package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Kyma Metrics", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", "regular",
		v1beta2.SyncStrategyLocalSecret)
	GinkgoWriter.Printf("kyma before create %v\n", kyma)

	InitEmptyKymaBeforeAll(kyma)

	It("Kyma reconciliation should remove metric when Kyma CR deleted ", func() {
		By("getting the current kyma Ready state metric count")
		kymaStateReadyCount, err := GetKymaStateMetricCount(ctx, kyma.GetName(), "Ready")
		Expect(err).Should(Not(HaveOccurred()))
		GinkgoWriter.Printf("Kyma State Ready Metric count before CR deletion: %d", kymaStateReadyCount)
		Expect(kymaStateReadyCount).Should(Equal(1))

		By("deleting KCP Kyma")
		Eventually(DeleteKymaByForceRemovePurgeFinalizer).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma).
			Should(Succeed())

		By("waiting for Kyma CR to be removed")
		Eventually(KymaDeleted).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
			Should(Succeed())

		By("should decrease the metric count")
		for _, state := range []string{"Deleting", "Warning", "Ready", "Processing", "Error"} {
			count, err := GetKymaStateMetricCount(ctx, kyma.GetName(), state)
			Expect(err).Should(Not(HaveOccurred()))
			GinkgoWriter.Printf("Kyma %s State Metric count after CR deletion: %d", state, count)
			Expect(count).Should(Equal(0))
		}
	})
})
