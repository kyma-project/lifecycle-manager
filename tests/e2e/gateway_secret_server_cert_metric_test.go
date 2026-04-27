package e2e_test

import (
	"time"

	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Gateway Secret Server Cert Close To Expiry Metric", Ordered, func() {
	Context("Given KCP cluster with an istio gateway secret", func() {
		It("When the server certificate is not close to expiry", func() {
			By("Then the metric is set to 0")
			Eventually(GetGatewaySecretServerCertCloseToExpiryGauge).
				WithContext(ctx).
				Should(Equal(0))
		})

		It("When the server certificate is manually replaced with one close to expiry", func() {
			By("And the gateway secret tls.crt is updated with an expiring certificate")
			Eventually(UpdateGatewaySecretWithExpiringCert).
				WithContext(ctx).
				WithArguments(kcpClient).
				Should(Succeed())

			By("Then the metric is set to 1")
			Eventually(GetGatewaySecretServerCertCloseToExpiryGauge).
				WithContext(ctx).
				WithTimeout(2 * time.Minute).
				Should(Equal(1))
		})
	})
})
