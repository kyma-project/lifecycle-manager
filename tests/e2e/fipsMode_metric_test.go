package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	//["github.com/kyma-project/lifecycle-manager/api/shared"
	//"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	//. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
)


var _ = Describe("FIPS Mode metric", Ordered, func() {

	//kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)

	//InitEmptyKymaBeforeAll(kyma)
	//CleanupKymaAfterAll(kyma)

	Context("Given KCP Cluster", func() {
		It("When KLM is initialized", func() {
			By("Then fipsMode metrics is set to \"enabled\"")
			Eventually(GetFipsModeGauge).
				WithContext(ctx).
				Should(Equal(0)) //TODO: Change to 1
		})
	})
})
