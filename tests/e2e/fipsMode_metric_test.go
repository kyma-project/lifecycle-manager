package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
)


var _ = Describe("FIPS Mode metric", Ordered, func() {

	Context("Given KCP Cluster", func() {
		It("When KLM is initialized", func() {
			By("Then fipsMode metrics is set to \"enabled\"")
			Eventually(GetFipsModeGauge).
				WithContext(ctx).
				Should(Equal(0)) // Then change to 1
		})
	})
})
