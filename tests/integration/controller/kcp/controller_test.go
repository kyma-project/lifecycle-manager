package kcp_test

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Kyma with managed fields in kcp mode", Ordered, func() {
	kyma := NewTestKyma("managed-kyma")
	kyma.Labels[shared.SyncLabel] = shared.DisableLabelValue

	registerControlPlaneLifecycleForKyma(kyma)

	It("Should result in a managed field with manager named 'lifecycle-manager'", func() {
		Eventually(ContainsKymaManagerField, Timeout, Interval).
			WithArguments(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace(), shared.OperatorName).
			Should(BeTrue())
	})
})
