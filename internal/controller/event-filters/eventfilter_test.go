package event_filters_test

import (
	. "github.com/kyma-project/lifecycle-manager/api/shared"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Kyma is reconciled correctly based on the event filters", Ordered, func() {
	kyma := NewTestKyma("kyma")
	BeforeAll(func() {
		Eventually(CreateCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma).Should(Succeed())

		Eventually(IsKymaInState, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, StateReady).
			Should(Succeed())
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Eventually(SyncKyma, Timeout, Interval).
			WithContext(ctx).WithArguments(controlPlaneClient, kyma).Should(Succeed())
	})

	Context("Given Kyma Controller is set with generation predicate event filter", func() {
		newChannel := "test"

		It("When kyma.spec is updated", func() {
			Eventually(updateKymaChannel).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma, newChannel).
				Should(Succeed())
		})

		It("Then Kyma should be reconciled immediately", func() {
			Eventually(kymaIsInExpectedStateWithUpdatedChannel).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma.Name, kyma.Namespace, newChannel, StateReady).
				Should(Succeed())
		})
	})

	Context("Given Kyma Controller is set with label predicate event filter", func() {
		labelKey := "new-label-key"
		labelValue := "label-value"
		It("When a new label is added to Kyma", func() {
			Eventually(addLabelToKyma).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma, labelKey, labelValue).
				Should(Succeed())
		})

		It("Then Kyma should be reconciled immediately", func() {
			Eventually(kymaIsInExpectedStateWithLabelUpdated).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma.Name, kyma.Namespace, labelKey, labelValue, StateReady).
				Should(Succeed())
		})
	})

	AfterAll(func() {
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma).Should(Succeed())
	})
})
