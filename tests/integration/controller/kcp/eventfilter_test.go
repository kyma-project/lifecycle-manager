package kcp_test

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Kyma is reconciled correctly based on the event filters", Ordered, func() {
	kyma := NewTestKyma("kyma")
	skrKyma := NewSKRKyma()
	var skrClient client.Client
	var err error

	BeforeAll(func() {
		Eventually(CreateCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).Should(Succeed())

		Eventually(func() error {
			skrClient, err = testSkrContextFactory.Get(kyma.GetNamespacedName())
			return err
		}, Timeout, Interval).Should(Succeed())

		Eventually(KymaIsInState, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
			Should(Succeed())
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Eventually(SyncKyma, Timeout, Interval).
			WithContext(ctx).WithArguments(kcpClient, kyma).Should(Succeed())
	})

	Context("Given Kyma Controller is set with generation predicate event filter", func() {
		newChannel := "fantastic"

		It("When kyma.spec is updated", func() {
			Eventually(updateKymaChannel).
				WithContext(ctx).
				WithArguments(skrClient, skrKyma.Name, skrKyma.Namespace, newChannel).
				Should(Succeed())
		})

		It("Then Kyma should be reconciled immediately", func() {
			Eventually(kymaIsInExpectedStateWithUpdatedChannel).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace, newChannel, shared.StateReady).
				Should(Succeed())
		})
	})

	Context("Given Kyma Controller is set with label predicate event filter", func() {
		labelKey := "new-label-key"
		labelValue := "label-value"
		It("When a new label is added to Kyma", func() {
			Eventually(addLabelToKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma, labelKey, labelValue).
				Should(Succeed())
		})

		It("Then Kyma should be reconciled immediately", func() {
			Eventually(kymaIsInExpectedStateWithLabelUpdated).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace, labelKey, labelValue, shared.StateReady).
				Should(Succeed())
		})
	})

	AfterAll(func() {
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).Should(Succeed())
	})
})
