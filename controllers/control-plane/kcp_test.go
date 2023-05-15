package control_plane_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Kyma with managed fields", Ordered, func() {
	kyma := NewTestKyma("managed-kyma")
	RegisterControlPlaneLifecycleForKyma(kyma)

	It("Should result in a managed field with manager named 'lifecycle-manager'", func() {
		Eventually(ExpectKymaManagerField, Timeout, Interval).
			WithArguments(ctx, controlPlaneClient, kyma.GetName(), v1beta2.OperatorName).
			Should(BeTrue())
	})
})
